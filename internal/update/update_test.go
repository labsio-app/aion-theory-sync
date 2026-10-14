package update

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func checkerAgainst(server *httptest.Server) *Checker {
	return &Checker{BaseURL: server.URL, HTTP: server.Client()}
}

func TestCheck_returnsNilWhenCurrentVersionIsNotSemver(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Check should not call the API for a non-semver current version")
	}))
	defer server.Close()

	got, err := checkerAgainst(server).Check(context.Background(), "dev")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got != nil {
		t.Fatalf("Check = %+v, want nil", got)
	}
}

func TestCheck_returnsNilWhenTheLatestReleaseIsNotNewer(t *testing.T) {
	cases := []string{"v0.2.0", "v0.1.9"}
	for _, latest := range cases {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(ghRelease{TagName: latest})
		}))

		got, err := checkerAgainst(server).Check(context.Background(), "v0.2.0")
		server.Close()
		if err != nil {
			t.Fatalf("latest %s: Check: %v", latest, err)
		}
		if got != nil {
			t.Fatalf("latest %s: Check = %+v, want nil", latest, got)
		}
	}
}

func TestCheck_returnsTheNewerReleaseAndItsAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/labsio-app/aion-theory-sync/releases/latest" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(ghRelease{
			TagName: "v0.3.0",
			Assets: []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			}{
				{Name: "aion-theory-sync_0.3.0_windows_amd64.exe", BrowserDownloadURL: "https://example.invalid/console.exe"},
				{Name: "aion-theory-sync-gui_0.3.0_windows_amd64.exe", BrowserDownloadURL: "https://example.invalid/gui.exe"},
				{Name: "aion-theory-sync-setup.exe", BrowserDownloadURL: "https://example.invalid/setup.exe"},
			},
		})
	}))
	defer server.Close()

	got, err := checkerAgainst(server).Check(context.Background(), "v0.2.0")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got == nil {
		t.Fatal("Check = nil, want the v0.3.0 release")
	}
	if got.Version != "v0.3.0" {
		t.Fatalf("Version = %q, want v0.3.0", got.Version)
	}
	if got.DownloadURL != "https://example.invalid/gui.exe" {
		t.Fatalf("DownloadURL = %q, want the gui asset's", got.DownloadURL)
	}
}

func TestCheck_errorsWhenTheNewerReleaseHasNoMatchingAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ghRelease{TagName: "v0.3.0"})
	}))
	defer server.Close()

	_, err := checkerAgainst(server).Check(context.Background(), "v0.2.0")
	if err == nil {
		t.Fatal("Check: want an error for a release with no gui asset")
	}
}

func TestDownload_reportsProgressUpToTheFullSize(t *testing.T) {
	const body = "this is a fake update exe, just needs a known length"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, body)
	}))
	defer server.Close()

	var calls []int64
	dest, err := checkerAgainst(server).Download(context.Background(), server.URL, func(downloaded, total int64) {
		calls = append(calls, downloaded)
		if total != int64(len(body)) {
			t.Errorf("total = %d, want %d", total, len(body))
		}
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	defer os.Remove(dest)

	if len(calls) == 0 {
		t.Fatal("onProgress was never called")
	}
	if got := calls[len(calls)-1]; got != int64(len(body)) {
		t.Fatalf("final downloaded = %d, want %d", got, len(body))
	}

	raw, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", dest, err)
	}
	if string(raw) != body {
		t.Fatalf("downloaded content = %q, want %q", raw, body)
	}
}

func TestDownload_worksWithoutAnOnProgressCallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "fake exe")
	}))
	defer server.Close()

	dest, err := checkerAgainst(server).Download(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	defer os.Remove(dest)

	raw, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", dest, err)
	}
	if !strings.Contains(string(raw), "fake exe") {
		t.Fatalf("downloaded content = %q", raw)
	}
}

func TestIsNewer(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"v0.2.0", "v0.3.0", true},
		{"v0.2.0", "v0.2.1", true},
		{"v0.2.0", "v1.0.0", true},
		{"v0.2.0", "v0.2.0", false},
		{"v0.2.0", "v0.1.9", false},
		{"v1.9.0", "v2.0.0", true},
	}
	for _, c := range cases {
		if got := isNewer(c.current, c.latest); got != c.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestIsSemver(t *testing.T) {
	cases := []struct {
		v    string
		want bool
	}{
		{"v1.2.3", true},
		{"1.2.3", true}, // no leading "v": TrimPrefix is a no-op, still valid
		{"dev", false},
		{"", false},
		{"v1", false},
		{"v1.2", false},
		{"v1.2.3-beta", false},
	}
	for _, c := range cases {
		if got := isSemver(c.v); got != c.want {
			t.Errorf("isSemver(%q) = %v, want %v", c.v, got, c.want)
		}
	}
}
