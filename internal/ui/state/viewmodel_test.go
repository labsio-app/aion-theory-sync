package state

import (
	"strings"
	"testing"
	"time"

	"github.com/labsio-app/aion-theory-sync/internal/api"
	"github.com/labsio-app/aion-theory-sync/internal/sync"
)

func TestBuildViewModel_unlinked(t *testing.T) {
	m := NewMachine(Unlinked)
	vm := BuildViewModel(m, time.Now())

	if vm.Unlinked.Title != "Lie cette application à ton compte Aion Theory" {
		t.Fatalf("Title = %q", vm.Unlinked.Title)
	}
	if vm.Unlinked.PrimaryButton != "Lier mon compte" {
		t.Fatalf("PrimaryButton = %q", vm.Unlinked.PrimaryButton)
	}
	if vm.Unlinked.ExpiredBanner {
		t.Fatal("ExpiredBanner = true on a fresh Unlinked machine")
	}
	if vm.Stepper[0].Status != StepCurrent || vm.Stepper[1].Status != StepUpcoming {
		t.Fatalf("Stepper = %+v, want step 0 current", vm.Stepper)
	}
}

func TestBuildViewModel_unlinkedShowsTheExpiredBanner(t *testing.T) {
	m := NewMachine(Linking)
	if err := m.GrantExpired(); err != nil {
		t.Fatalf("GrantExpired: %v", err)
	}
	vm := BuildViewModel(m, time.Now())
	if !vm.Unlinked.ExpiredBanner {
		t.Fatal("ExpiredBanner = false, want true")
	}
	if !strings.Contains(vm.Unlinked.ExpiredBannerText, "expiré") {
		t.Fatalf("ExpiredBannerText = %q", vm.Unlinked.ExpiredBannerText)
	}
}

func TestBuildViewModel_linkingShowsTheCodeAndCountdown(t *testing.T) {
	m := NewMachine(Linking)
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	if err := m.GrantOpened(api.DeviceGrant{
		UserCode:        "ABCD-1234",
		VerificationURL: "https://aion-theory.example/activate",
		ExpiresAt:       now.Add(5 * time.Minute).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("GrantOpened: %v", err)
	}

	vm := BuildViewModel(m, now)
	if vm.Linking.UserCode != "ABCD-1234" {
		t.Fatalf("UserCode = %q", vm.Linking.UserCode)
	}
	if vm.Linking.Countdown != "05:00" {
		t.Fatalf("Countdown = %q, want 05:00", vm.Linking.Countdown)
	}
	if !strings.Contains(vm.Linking.WaitingText, "05:00") {
		t.Fatalf("WaitingText = %q", vm.Linking.WaitingText)
	}
	if vm.Linking.VerificationURL != "https://aion-theory.example/activate" {
		t.Fatalf("VerificationURL = %q", vm.Linking.VerificationURL)
	}
	if vm.Linking.Expired {
		t.Fatal("Expired = true before the deadline")
	}

	later := now.Add(10 * time.Minute)
	vmLater := BuildViewModel(m, later)
	if !vmLater.Linking.Expired {
		t.Fatal("Expired = false after the deadline has passed")
	}
	if vmLater.Linking.Countdown != "00:00" {
		t.Fatalf("Countdown past the deadline = %q, want 00:00", vmLater.Linking.Countdown)
	}
}

func TestBuildViewModel_prerequisitesWithTheMeterFound(t *testing.T) {
	m := NewMachine(Prerequisites)
	m.DPSMeter = DPSMeterInfo{Found: true, PendingCount: 1}

	vm := BuildViewModel(m, time.Now())
	if vm.Prerequisites.DPSMeterCard.Subtitle != "Installé · 1 combat trouvé" {
		t.Fatalf("Subtitle = %q", vm.Prerequisites.DPSMeterCard.Subtitle)
	}
	if !vm.Prerequisites.DPSMeterCard.Done {
		t.Fatal("DPSMeterCard.Done = false with the meter found")
	}
	if !vm.Prerequisites.FinishEnabled {
		t.Fatal("FinishEnabled = false with the meter found")
	}
	if vm.Prerequisites.CharacterVisible {
		t.Fatal("CharacterVisible = true, want it hidden (no characters endpoint yet)")
	}
}

func TestBuildViewModel_prerequisitesWithTheMeterMissing(t *testing.T) {
	m := NewMachine(Prerequisites)
	m.DPSMeter = DPSMeterInfo{Found: false}

	vm := BuildViewModel(m, time.Now())
	if vm.Prerequisites.DPSMeterCard.Subtitle != "Introuvable" {
		t.Fatalf("Subtitle = %q", vm.Prerequisites.DPSMeterCard.Subtitle)
	}
	if vm.Prerequisites.FinishEnabled {
		t.Fatal("FinishEnabled = true with the meter missing")
	}
	if vm.Prerequisites.DPSMeterCard.ActionURL != "https://a2tools.app" {
		t.Fatalf("ActionURL = %q", vm.Prerequisites.DPSMeterCard.ActionURL)
	}
}

func TestBuildViewModel_readyShowsThePendingCountSingularAndPlural(t *testing.T) {
	m := NewMachine(Ready)
	m.DPSMeter = DPSMeterInfo{PendingCount: 1}
	m.LastSyncedAt = time.Date(2026, 10, 14, 17, 55, 0, 0, time.UTC)
	now := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)

	vm := BuildViewModel(m, now)
	if vm.Ready.PendingLabel != "nouveau combat prêt à être envoyé" {
		t.Fatalf("PendingLabel (singular) = %q", vm.Ready.PendingLabel)
	}

	m.DPSMeter.PendingCount = 3
	vm = BuildViewModel(m, now)
	if vm.Ready.PendingLabel != "nouveaux combats prêts à être envoyés" {
		t.Fatalf("PendingLabel (plural) = %q", vm.Ready.PendingLabel)
	}
	if !strings.HasPrefix(vm.Ready.LastSyncedLabel, "Dernière synchro : ") {
		t.Fatalf("LastSyncedLabel = %q", vm.Ready.LastSyncedLabel)
	}
}

func TestBuildViewModel_syncingShowsProgressAndCurrentFile(t *testing.T) {
	m := NewMachine(Syncing)
	// Built in time.Local, not time.UTC: CurrentFileLabel renders the
	// clock time in local time, so the expected string below must not
	// assume the test machine's offset is zero (see RelativeDate's test).
	startedAt := time.Date(2026, 10, 14, 21, 42, 0, 0, time.Local)
	m.SyncProgress = ProgressInfo{
		Total: 10, Done: 4,
		Current: CurrentFile{
			StartedAt: startedAt,
			Duration:  3*time.Minute + 12*time.Second,
		},
	}

	vm := BuildViewModel(m, time.Now())
	if vm.Syncing.Done != 4 || vm.Syncing.Total != 10 {
		t.Fatalf("Done/Total = %d/%d, want 4/10", vm.Syncing.Done, vm.Syncing.Total)
	}
	want := "Combat du 14 oct. à " + startedAt.Format("15:04") + " · 3 min 12 s"
	if vm.Syncing.CurrentFileLabel != want {
		t.Fatalf("CurrentFileLabel = %q, want %q", vm.Syncing.CurrentFileLabel, want)
	}
	if vm.Syncing.PauseResumeButton != "Mettre en pause" {
		t.Fatalf("PauseResumeButton = %q, want Mettre en pause", vm.Syncing.PauseResumeButton)
	}

	m.Paused = true
	vm = BuildViewModel(m, time.Now())
	if vm.Syncing.PauseResumeButton != "Reprendre" {
		t.Fatalf("PauseResumeButton = %q, want Reprendre once paused", vm.Syncing.PauseResumeButton)
	}
}

func TestBuildViewModel_syncingShowsTheRateLimitCountdown(t *testing.T) {
	m := NewMachine(Syncing)
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	if err := m.RateLimited(30*time.Second, now); err != nil {
		t.Fatalf("RateLimited: %v", err)
	}

	vm := BuildViewModel(m, now.Add(10*time.Second))
	if !vm.Syncing.RateLimitActive {
		t.Fatal("RateLimitActive = false")
	}
	want := "Le serveur demande une courte pause. Reprise automatique dans 20 s."
	if vm.Syncing.RateLimitText != want {
		t.Fatalf("RateLimitText = %q, want %q", vm.Syncing.RateLimitText, want)
	}
}

func TestBuildViewModel_doneSummarizesEveryTile(t *testing.T) {
	m := NewMachine(Syncing)
	summary := sync.Summary{
		Recorded:        5,
		AlreadyRecorded: 2,
		Skipped:         map[string]int{"UNREADABLE": 1},
		Refused:         map[string]int{"NO_MATCHING_CHARACTER": 3},
		Failed:          1,
	}
	if err := m.SyncFinished(summary, 0); err != nil {
		t.Fatalf("SyncFinished: %v", err)
	}

	vm := BuildViewModel(m, time.Now())
	if vm.Done.Title != "5 combats envoyés" {
		t.Fatalf("Title = %q", vm.Done.Title)
	}
	if vm.Done.AlreadyPresent.Count != 2 {
		t.Fatalf("AlreadyPresent.Count = %d, want 2", vm.Done.AlreadyPresent.Count)
	}
	if vm.Done.NoMatchingCharacter.Count != 3 || !vm.Done.NoMatchingCharacter.Warning {
		t.Fatalf("NoMatchingCharacter = %+v, want count 3 with a warning border", vm.Done.NoMatchingCharacter)
	}
	if vm.Done.Unreadable.Count != 1 {
		t.Fatalf("Unreadable.Count = %d, want 1", vm.Done.Unreadable.Count)
	}
	if !vm.Done.FollowupVisible || !strings.Contains(vm.Done.FollowupText, "3 combats seront repris automatiquement") {
		t.Fatalf("FollowupText = %q (visible=%v)", vm.Done.FollowupText, vm.Done.FollowupVisible)
	}
	if !vm.Done.FailedBannerVisible || !strings.Contains(vm.Done.FailedBannerText, "1 combat n'a pas pu partir") {
		t.Fatalf("FailedBannerText = %q (visible=%v)", vm.Done.FailedBannerText, vm.Done.FailedBannerVisible)
	}
	if vm.Done.SubtitleVisible {
		t.Fatal("SubtitleVisible = true, want it hidden (no declared-character name available)")
	}
}

func TestBuildViewModel_doneSingularTitleAndFollowup(t *testing.T) {
	m := NewMachine(Syncing)
	summary := sync.Summary{Recorded: 1, Refused: map[string]int{"AMBIGUOUS_MATCH": 1}}
	if err := m.SyncFinished(summary, 0); err != nil {
		t.Fatalf("SyncFinished: %v", err)
	}
	vm := BuildViewModel(m, time.Now())
	if vm.Done.Title != "1 combat envoyé" {
		t.Fatalf("Title = %q, want singular", vm.Done.Title)
	}
	if !strings.Contains(vm.Done.FollowupText, "1 combat sera repris automatiquement") {
		t.Fatalf("FollowupText = %q, want singular agreement", vm.Done.FollowupText)
	}
}

func TestBuildViewModel_upToDate(t *testing.T) {
	m := NewMachine(UpToDate)
	vm := BuildViewModel(m, time.Now())
	if vm.UpToDate.Title != "Tout est à jour" {
		t.Fatalf("Title = %q", vm.UpToDate.Title)
	}
	if !vm.UpToDate.SyncNowDisabled {
		t.Fatal("SyncNowDisabled = false on the up-to-date screen")
	}
}

func TestBuildViewModel_blockedDPSMeterMissing(t *testing.T) {
	m := NewMachine(Blocked)
	m.Block = BlockInfo{Reason: BlockDPSMeterMissing}
	vm := BuildViewModel(m, time.Now())
	if vm.Blocked.Title != "On ne trouve pas les combats du DPS Meter A2Tools" {
		t.Fatalf("Title = %q", vm.Blocked.Title)
	}
	if vm.Blocked.PrimaryButton != "Choisir le dossier" {
		t.Fatalf("PrimaryButton = %q", vm.Blocked.PrimaryButton)
	}
	if vm.Blocked.InstallLinkText == "" {
		t.Fatal("InstallLinkText is empty for a missing-meter block")
	}
}

func TestBuildViewModel_blockedNetwork(t *testing.T) {
	m := NewMachine(Blocked)
	m.Block = BlockInfo{Reason: BlockNetwork}
	vm := BuildViewModel(m, time.Now())
	if vm.Blocked.PrimaryButton != "" {
		t.Fatalf("PrimaryButton = %q, want none for a network block", vm.Blocked.PrimaryButton)
	}
	if vm.Blocked.RetryButton != "Réessayer" {
		t.Fatalf("RetryButton = %q", vm.Blocked.RetryButton)
	}
}
