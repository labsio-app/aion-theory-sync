package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/labsio-app/aion-theory-sync/internal/api"
)

// pollInterval is fixed rather than server-dictated: the device grant has
// no slow_down signal of its own (platform-side rate limit is generous
// enough, L4), and a fixed 4s interval comfortably clears its 15 minute TTL.
// Tests shrink it to avoid waiting on the real-world cadence.
var pollInterval = 4 * time.Second

// EnsureCredential returns a usable credential: the one on disk if there is
// one, otherwise a fresh device flow run to completion (T1). The caller is
// responsible for calling ClearCredential and re-running this after an
// upload is rejected with 401 (F16).
func EnsureCredential(ctx context.Context, client *api.Client, dir string, out io.Writer) (string, error) {
	stored, err := LoadCredential(dir)
	if err != nil {
		return "", err
	}
	if stored != "" {
		return stored, nil
	}
	return RunDeviceFlow(ctx, client, dir, out)
}

// RunDeviceFlow opens a device grant, shows the member the code and the
// link, and polls until it is approved, denied or expired.
func RunDeviceFlow(ctx context.Context, client *api.Client, dir string, out io.Writer) (string, error) {
	grant, err := client.OpenDevice(ctx)
	if err != nil {
		return "", fmt.Errorf("opening a device grant: %w", err)
	}

	fmt.Fprintf(out, "Open %s and enter this code: %s\n", grant.VerificationURL, grant.UserCode)
	fmt.Fprintln(out, "Waiting for approval...")

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			poll, err := client.PollToken(ctx, grant.GrantID)
			if err != nil {
				// Only a terminal outcome stops the flow (F16): a transient
				// network error is worth retrying on the next tick, exactly
				// as the client's own PollToken doc comment promises.
				if errors.Is(err, api.ErrExpired) || errors.Is(err, api.ErrNotFound) ||
					errors.Is(err, api.ErrAlreadyUsed) {
					return "", fmt.Errorf("the device code expired or was already used: %w", err)
				}
				fmt.Fprintf(out, "Could not reach the server, retrying: %v\n", err)
				continue
			}
			if poll.Status == "pending" {
				continue
			}
			if err := SaveCredential(dir, poll.Credential); err != nil {
				return "", fmt.Errorf("saving the issued credential: %w", err)
			}
			fmt.Fprintln(out, "Linked. This device is now authorized to sync.")
			return poll.Credential, nil
		}
	}
}
