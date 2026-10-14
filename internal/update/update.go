// Package update checks GitHub for a newer release of this tool and, if
// the member agrees, fetches and installs it in place — so a member never
// has to go back to the releases page and download the app again by hand.
// It only concerns the installable GUI build (cmd/gui); the console tool
// has no UI to drive a prompt from and is expected to be reinstalled like
// any other CLI.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// repo is the GitHub repository releases are checked against — the same
// one .goreleaser.yml and .github/workflows/release.yml publish to.
const repo = "labsio-app/aion-theory-sync"

// assetPattern is the one release asset this checks for: the raw,
// unzipped windows/amd64 GUI binary goreleaser publishes directly
// (formats: [binary] in .goreleaser.yml) — never the console tool's own
// binary or the signed Setup.exe installer, which both ship as separate
// assets on the same release.
var assetPattern = regexp.MustCompile(`^aion-theory-sync-gui_.*_windows_amd64\.exe$`)

// Available describes a newer release this tool knows how to fetch and
// install in place.
type Available struct {
	Version     string // e.g. "v0.2.0", exactly as the git tag names it
	DownloadURL string
}

// Checker asks GitHub for the latest release. BaseURL defaults to the
// real GitHub API and only needs overriding in tests.
type Checker struct {
	BaseURL string
	HTTP    *http.Client
}

// New returns a Checker pointed at the real GitHub API. It carries no
// client-level Timeout: Check and Download each take their own ctx, and
// an http.Client.Timeout would otherwise cap Download's transfer too —
// a slow connection downloading the exe could then be cut off well
// before its own, longer-lived ctx expires.
func New() *Checker {
	return &Checker{BaseURL: "https://api.github.com", HTTP: &http.Client{}}
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// Check reports the latest release only when it is both a real semantic
// version strictly newer than currentVersion and carries the asset this
// tool knows how to install; otherwise it returns (nil, nil). A
// currentVersion that isn't a valid "vMAJOR.MINOR.PATCH" (e.g. "dev", any
// build that skipped -ldflags) never matches and short-circuits before
// the request, rather than nagging a local build with every release ever
// published.
func (c *Checker) Check(ctx context.Context, currentVersion string) (*Available, error) {
	if !isSemver(currentVersion) {
		return nil, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github releases/latest: unexpected status %d", resp.StatusCode)
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}

	if !isSemver(rel.TagName) || !isNewer(currentVersion, rel.TagName) {
		return nil, nil
	}

	for _, asset := range rel.Assets {
		if assetPattern.MatchString(asset.Name) {
			return &Available{Version: rel.TagName, DownloadURL: asset.BrowserDownloadURL}, nil
		}
	}
	return nil, fmt.Errorf("release %s has no asset matching %s", rel.TagName, assetPattern)
}

// Download streams url to a new file next to the running exe, named after
// it with ".update.exe" appended — kept on the same volume as the running
// exe so the rename in Apply can't fail across filesystems.
//
// onProgress, if not nil, is called after every chunk read with the bytes
// copied so far and the total from the response's Content-Length — the
// GUI's one hook for an actual progress bar instead of a static "please
// wait" message. total is <= 0 when the server didn't report a length;
// the caller should fall back to an indeterminate indicator in that case.
func (c *Checker) Download(ctx context.Context, url string, onProgress func(downloaded, total int64)) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dest := exe + ".update.exe"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading update: unexpected status %d", resp.StatusCode)
	}

	f, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return "", err
	}

	var body io.Reader = resp.Body
	if onProgress != nil {
		body = &progressReader{r: resp.Body, total: resp.ContentLength, onProgress: onProgress}
	}

	if _, err := io.Copy(f, body); err != nil {
		f.Close()
		os.Remove(dest)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(dest)
		return "", err
	}
	return dest, nil
}

// progressReader reports cumulative bytes read after every underlying
// Read, piggybacking on io.Copy's own chunking instead of introducing a
// second buffering pass just to observe the byte count.
type progressReader struct {
	r          io.Reader
	total      int64
	downloaded int64
	onProgress func(downloaded, total int64)
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.r.Read(buf)
	p.downloaded += int64(n)
	p.onProgress(p.downloaded, p.total)
	return n, err
}

// Apply swaps the freshly downloaded exe (newExePath, from Download) in
// for the one currently running. Windows allows renaming a running exe —
// it only blocks deleting or overwriting its bytes in place — so this
// needs no elevation and no separate helper process. The caller still has
// to Relaunch and exit afterward for the new binary to actually run.
//
// The old exe is left behind as ".old.exe" rather than deleted outright,
// since Windows may still hold its file handle open for a moment after
// this process exits; RemoveOldVersion cleans it up on the next launch.
func Apply(newExePath string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	old := exe + ".old.exe"
	_ = os.Remove(old) // best-effort: a leftover from a previous update

	if err := os.Rename(exe, old); err != nil {
		return fmt.Errorf("moving the running exe aside: %w", err)
	}
	if err := os.Rename(newExePath, exe); err != nil {
		_ = os.Rename(old, exe) // best-effort rollback so the app isn't left missing its exe
		return fmt.Errorf("installing the downloaded update: %w", err)
	}
	return nil
}

// RemoveOldVersion deletes the ".old.exe" a previous Apply left behind, if
// any. Meant to be called once at startup, not right after Relaunch: the
// exiting old process may still hold the file open for a moment.
func RemoveOldVersion() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	_ = os.Remove(exe + ".old.exe")
}

// Relaunch starts a new, detached process from the exe's own current
// path. The caller is still responsible for exiting afterward.
func Relaunch() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return exec.Command(exe).Start()
}

func isSemver(v string) bool {
	_, _, _, ok := parseSemver(v)
	return ok
}

// isNewer reports whether latest is a strictly greater semantic version
// than current. Both must already be valid per parseSemver.
func isNewer(current, latest string) bool {
	cMaj, cMin, cPatch, _ := parseSemver(current)
	lMaj, lMin, lPatch, _ := parseSemver(latest)
	if lMaj != cMaj {
		return lMaj > cMaj
	}
	if lMin != cMin {
		return lMin > cMin
	}
	return lPatch > cPatch
}

// parseSemver reads the "vMAJOR.MINOR.PATCH" tags this project's releases
// use (release.yml triggers on the "v*" tag pattern); anything else,
// including a pre-release or build suffix, is rejected rather than
// guessed at.
func parseSemver(v string) (major, minor, patch int, ok bool) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return 0, 0, 0, false
		}
		nums[i] = n
	}
	return nums[0], nums[1], nums[2], true
}
