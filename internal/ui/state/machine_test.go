package state

import (
	"errors"
	"testing"
	"time"

	"github.com/labsio-app/aion-theory-sync/internal/api"
	"github.com/labsio-app/aion-theory-sync/internal/sync"
)

func TestDeriveIdleState(t *testing.T) {
	cases := []struct {
		name          string
		hasCredential bool
		meterFound    bool
		pending       int
		want          State
	}{
		{"no credential at all", false, true, 5, Unlinked},
		{"linked but meter missing", true, false, 0, Blocked},
		{"linked, meter found, nothing pending", true, true, 0, UpToDate},
		{"linked, meter found, combats pending", true, true, 3, Ready},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DeriveIdleState(c.hasCredential, c.meterFound, c.pending); got != c.want {
				t.Fatalf("DeriveIdleState(%v, %v, %d) = %s, want %s", c.hasCredential, c.meterFound, c.pending, got, c.want)
			}
		})
	}
}

func TestMachine_onboardingHappyPath(t *testing.T) {
	m := NewMachine(Unlinked)

	if err := m.LinkRequested(); err != nil {
		t.Fatalf("LinkRequested: %v", err)
	}
	if m.Current != Linking {
		t.Fatalf("Current = %s, want Linking", m.Current)
	}

	grant := api.DeviceGrant{
		UserCode:        "ABCD-1234",
		VerificationURL: "https://aion-theory.example/activate",
		ExpiresAt:       "2026-10-15T12:15:00Z",
	}
	if err := m.GrantOpened(grant); err != nil {
		t.Fatalf("GrantOpened: %v", err)
	}
	if m.Current != Linking {
		t.Fatalf("Current = %s, want still Linking", m.Current)
	}
	if m.Grant.UserCode != "ABCD-1234" {
		t.Fatalf("Grant.UserCode = %q, want ABCD-1234", m.Grant.UserCode)
	}
	wantExpiry, _ := time.Parse(time.RFC3339, "2026-10-15T12:15:00Z")
	if !m.Grant.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("Grant.ExpiresAt = %v, want %v", m.Grant.ExpiresAt, wantExpiry)
	}

	if err := m.GrantApproved(); err != nil {
		t.Fatalf("GrantApproved: %v", err)
	}
	if m.Current != Prerequisites {
		t.Fatalf("Current = %s, want Prerequisites", m.Current)
	}

	if err := m.PrereqsOK(DPSMeterInfo{Found: true, PendingCount: 4}); err != nil {
		t.Fatalf("PrereqsOK: %v", err)
	}
	if m.Current != Ready {
		t.Fatalf("Current = %s, want Ready (4 pending)", m.Current)
	}

	if err := m.SyncStarted(); err != nil {
		t.Fatalf("SyncStarted: %v", err)
	}
	if m.Current != Syncing {
		t.Fatalf("Current = %s, want Syncing", m.Current)
	}

	now := time.Date(2026, 10, 15, 20, 0, 0, 0, time.UTC)
	if err := m.Progress(ProgressInfo{Total: 4, Done: 1}); err != nil {
		t.Fatalf("Progress: %v", err)
	}
	if err := m.RateLimited(30*time.Second, now); err != nil {
		t.Fatalf("RateLimited: %v", err)
	}
	if !m.RateLimit.Active {
		t.Fatal("RateLimit.Active = false, want true")
	}
	if !m.RateLimit.RetryAt.Equal(now.Add(30 * time.Second)) {
		t.Fatalf("RateLimit.RetryAt = %v, want %v", m.RateLimit.RetryAt, now.Add(30*time.Second))
	}
	if err := m.Resumed(); err != nil {
		t.Fatalf("Resumed: %v", err)
	}
	if m.RateLimit.Active {
		t.Fatal("RateLimit.Active = true after Resumed, want false")
	}

	summary := sync.Summary{Recorded: 3, AlreadyRecorded: 1}
	if err := m.SyncFinished(summary, 0); err != nil {
		t.Fatalf("SyncFinished: %v", err)
	}
	if m.Current != Done {
		t.Fatalf("Current = %s, want Done", m.Current)
	}
	if m.Summary.Recorded != 3 || m.Summary.AlreadyRecorded != 1 {
		t.Fatalf("Summary = %+v, want Recorded=3 AlreadyRecorded=1", m.Summary)
	}
	if m.DPSMeter.PendingCount != 0 {
		t.Fatalf("DPSMeter.PendingCount = %d, want 0", m.DPSMeter.PendingCount)
	}
}

func TestMachine_prereqsOKLandsOnUpToDateWhenNothingIsPending(t *testing.T) {
	m := NewMachine(Prerequisites)
	if err := m.PrereqsOK(DPSMeterInfo{Found: true, PendingCount: 0}); err != nil {
		t.Fatalf("PrereqsOK: %v", err)
	}
	if m.Current != UpToDate {
		t.Fatalf("Current = %s, want UpToDate", m.Current)
	}
}

func TestMachine_grantExpiryReturnsToUnlinkedWithABanner(t *testing.T) {
	m := NewMachine(Linking)
	if err := m.GrantExpired(); err != nil {
		t.Fatalf("GrantExpired: %v", err)
	}
	if m.Current != Unlinked {
		t.Fatalf("Current = %s, want Unlinked", m.Current)
	}
	if !m.ExpiredBanner {
		t.Fatal("ExpiredBanner = false, want true")
	}
}

func TestMachine_cancellingLinkingClearsNoBanner(t *testing.T) {
	m := NewMachine(Linking)
	if err := m.UnlinkRequested(); err != nil {
		t.Fatalf("UnlinkRequested: %v", err)
	}
	if m.Current != Unlinked {
		t.Fatalf("Current = %s, want Unlinked", m.Current)
	}
	if m.ExpiredBanner || m.RevokedBanner {
		t.Fatalf("banners = expired:%v revoked:%v, want both false on a plain cancel", m.ExpiredBanner, m.RevokedBanner)
	}
}

func TestMachine_linkingAgainClearsTheExpiredBanner(t *testing.T) {
	m := NewMachine(Linking)
	if err := m.GrantExpired(); err != nil {
		t.Fatalf("GrantExpired: %v", err)
	}
	if err := m.LinkRequested(); err != nil {
		t.Fatalf("LinkRequested: %v", err)
	}
	if m.ExpiredBanner {
		t.Fatal("ExpiredBanner still true after relinking")
	}
}

func TestMachine_syncFailedWithARevokedCredentialReturnsToUnlinked(t *testing.T) {
	m := NewMachine(Syncing)
	if err := m.SyncFailed(api.ErrUnauthorized); err != nil {
		t.Fatalf("SyncFailed: %v", err)
	}
	if m.Current != Unlinked {
		t.Fatalf("Current = %s, want Unlinked", m.Current)
	}
	if !m.RevokedBanner {
		t.Fatal("RevokedBanner = false, want true")
	}
}

func TestMachine_syncFailedWithANetworkErrorGoesToBlocked(t *testing.T) {
	m := NewMachine(Syncing)
	cause := errors.New("dial tcp: no route to host")
	if err := m.SyncFailed(cause); err != nil {
		t.Fatalf("SyncFailed: %v", err)
	}
	if m.Current != Blocked {
		t.Fatalf("Current = %s, want Blocked", m.Current)
	}
	if m.Block.Reason != BlockNetwork {
		t.Fatalf("Block.Reason = %s, want %s", m.Block.Reason, BlockNetwork)
	}
	if !errors.Is(m.Block.Err, cause) {
		t.Fatalf("Block.Err = %v, want it to wrap %v", m.Block.Err, cause)
	}
}

func TestMachine_retryingFromBlockedCanReachReadyOrUpToDate(t *testing.T) {
	m := NewMachine(Blocked)
	m.Block = BlockInfo{Reason: BlockDPSMeterMissing}

	if err := m.PrereqsOK(DPSMeterInfo{Found: true, PendingCount: 2}); err != nil {
		t.Fatalf("PrereqsOK: %v", err)
	}
	if m.Current != Ready {
		t.Fatalf("Current = %s, want Ready", m.Current)
	}
}

func TestMachine_unlinkRequestedResetsEverything(t *testing.T) {
	m := NewMachine(Done)
	m.Summary = SummaryInfo{Recorded: 5}
	m.Character = CharacterInfo{Known: true, Name: "Leftover"}

	if err := m.UnlinkRequested(); err != nil {
		t.Fatalf("UnlinkRequested: %v", err)
	}
	if m.Current != Unlinked {
		t.Fatalf("Current = %s, want Unlinked", m.Current)
	}
	if m.Summary != (SummaryInfo{}) {
		t.Fatalf("Summary = %+v, want zero value after unlinking", m.Summary)
	}
	if m.Character != (CharacterInfo{}) {
		t.Fatalf("Character = %+v, want zero value after unlinking", m.Character)
	}
}

func TestMachine_settingsRoundTripReturnsToWhateverWasShowing(t *testing.T) {
	m := NewMachine(Ready)
	if err := m.SettingsOpened(); err != nil {
		t.Fatalf("SettingsOpened: %v", err)
	}
	if m.Current != Settings {
		t.Fatalf("Current = %s, want Settings", m.Current)
	}
	if err := m.SettingsClosed(); err != nil {
		t.Fatalf("SettingsClosed: %v", err)
	}
	if m.Current != Ready {
		t.Fatalf("Current = %s, want back to Ready", m.Current)
	}
}

func TestMachine_settingsClosedOutsideSettingsIsRejected(t *testing.T) {
	m := NewMachine(Ready)
	if err := m.SettingsClosed(); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("err = %v, want ErrInvalidTransition", err)
	}
}

func TestMachine_anEventInvalidForTheCurrentStateIsRejectedAndIgnored(t *testing.T) {
	m := NewMachine(Ready)
	before := *m
	if err := m.GrantApproved(); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("err = %v, want ErrInvalidTransition", err)
	}
	if m.Current != before.Current {
		t.Fatalf("Current changed on a rejected event: %s -> %s", before.Current, m.Current)
	}
}

func TestNewSummaryInfo_splitsRefusedReasonsBetweenTheNoMatchAndUnreadableTiles(t *testing.T) {
	m := NewMachine(Syncing)
	summary := sync.Summary{
		Recorded:        2,
		AlreadyRecorded: 1,
		Skipped:         map[string]int{"UNREADABLE": 1},
		Refused: map[string]int{
			"NO_MATCHING_CHARACTER": 2,
			"AMBIGUOUS_MATCH":       1,
			"OVERSIZED":             1,
		},
		Failed: 1,
	}
	if err := m.SyncFinished(summary, 0); err != nil {
		t.Fatalf("SyncFinished: %v", err)
	}
	want := SummaryInfo{
		Recorded:            2,
		AlreadyRecorded:     1,
		NoMatchingCharacter: 3, // NO_MATCHING_CHARACTER + AMBIGUOUS_MATCH
		Unreadable:          2, // the skipped file + OVERSIZED
		Failed:              1,
	}
	if m.Summary != want {
		t.Fatalf("Summary = %+v, want %+v", m.Summary, want)
	}
}
