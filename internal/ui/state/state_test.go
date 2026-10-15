package state

import "testing"

func TestNext_everyDefinedTransition(t *testing.T) {
	cases := []struct {
		name  string
		from  State
		event Event
		want  State
	}{
		{"link requested from unlinked opens linking", Unlinked, LinkRequested, Linking},

		{"grant opened while linking stays linking", Linking, GrantOpened, Linking},
		{"grant approved moves to prerequisites", Linking, GrantApproved, Prerequisites},
		{"grant expired returns to unlinked", Linking, GrantExpired, Unlinked},
		{"cancelling linking returns to unlinked", Linking, UnlinkRequested, Unlinked},

		{"prereqs ok leaves prerequisites", Prerequisites, PrereqsOK, Ready},
		{"prereqs missing stays on prerequisites", Prerequisites, PrereqsMissing, Prerequisites},

		{"starting a sync from ready", Ready, SyncStarted, Syncing},
		{"opening settings from ready", Ready, SettingsOpened, Settings},

		{"auto-sync can start from up to date", UpToDate, SyncStarted, Syncing},
		{"opening settings from up to date", UpToDate, SettingsOpened, Settings},

		{"progress update stays syncing", Syncing, Progress, Syncing},
		{"rate limited stays syncing", Syncing, RateLimited, Syncing},
		{"resumed stays syncing", Syncing, Resumed, Syncing},
		{"sync finished reaches done", Syncing, SyncFinished, Done},
		{"sync failed reaches blocked", Syncing, SyncFailed, Blocked},

		{"opening settings from done", Done, SettingsOpened, Settings},
		{"unlinking from done", Done, UnlinkRequested, Unlinked},

		{"retrying prereqs ok from blocked", Blocked, PrereqsOK, Ready},
		{"retrying prereqs missing from blocked", Blocked, PrereqsMissing, Blocked},
		{"unlinking from blocked", Blocked, UnlinkRequested, Unlinked},
		{"opening settings from blocked", Blocked, SettingsOpened, Settings},

		{"unlinking from settings", Settings, UnlinkRequested, Unlinked},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := Next(c.from, c.event)
			if !ok {
				t.Fatalf("Next(%s, %s) was not found in the table", c.from, c.event)
			}
			if got != c.want {
				t.Fatalf("Next(%s, %s) = %s, want %s", c.from, c.event, got, c.want)
			}
		})
	}
}

func TestNext_undefinedPairsAreRejected(t *testing.T) {
	cases := []struct {
		name  string
		from  State
		event Event
	}{
		{"cannot start a sync from unlinked", Unlinked, SyncStarted},
		{"cannot report progress outside syncing", Ready, Progress},
		{"cannot approve a grant outside linking", Unlinked, GrantApproved},
		{"cannot unlink from prerequisites", Prerequisites, UnlinkRequested},
		{"cannot unlink from unlinked", Unlinked, UnlinkRequested},
		{"settings has no sync-started handler", Settings, SyncStarted},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, ok := Next(c.from, c.event); ok {
				t.Fatalf("Next(%s, %s) was unexpectedly defined", c.from, c.event)
			}
		})
	}
}
