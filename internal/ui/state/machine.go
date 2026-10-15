package state

import (
	"errors"
	"fmt"
	"time"

	"github.com/labsio-app/aion-theory-sync/internal/api"
	"github.com/labsio-app/aion-theory-sync/internal/sync"
)

// ErrInvalidTransition is returned by every Machine method when its event
// is not valid in the machine's current state (see Next in state.go).
// cmd/gui is not expected to branch on it: every call site either comes
// from a widget that was already enabled/disabled to match the state (so
// the error can't actually happen), or is a background goroutine result
// that arrived late and should just be dropped.
var ErrInvalidTransition = errors.New("event is not valid in the current state")

// Machine is the stateful wrapper around the pure table in state.go: it
// holds the current screen plus whatever data that screen (and its
// view-model, see viewmodel.go) needs to render. Every exported field is
// read-only from cmd/gui's point of view — the methods below are the only
// way this package expects it to change, which is what keeps cmd/gui's own
// code free of business "if"s: it calls an event method and re-renders
// from BuildViewModel(m, time.Now()), nothing else.
type Machine struct {
	Current  State
	Previous State // only meaningful while Current == Settings; see SettingsClosed

	Grant        GrantInfo
	DPSMeter     DPSMeterInfo
	Character    CharacterInfo
	SyncProgress ProgressInfo
	RateLimit    RateLimitInfo
	Summary      SummaryInfo
	Block        BlockInfo

	ExpiredBanner bool // screen 0: "Le code a expiré. Relance la liaison."
	RevokedBanner bool // screen 0: shown after a 401 mid-sync
}

// NewMachine starts a Machine already on the given screen — normally the
// result of DeriveIdleState, run once at launch.
func NewMachine(initial State) *Machine {
	return &Machine{Current: initial}
}

func (m *Machine) fire(event Event) error {
	to, ok := Next(m.Current, event)
	if !ok {
		return fmt.Errorf("%s in %s: %w", event, m.Current, ErrInvalidTransition)
	}
	m.Current = to
	return nil
}

// LinkRequested starts linking: "Lier mon compte" on screen 0.
func (m *Machine) LinkRequested() error {
	if err := m.fire(LinkRequested); err != nil {
		return err
	}
	m.ExpiredBanner = false
	m.RevokedBanner = false
	return nil
}

// GrantOpened reports a freshly opened device grant, before polling begins.
func (m *Machine) GrantOpened(grant api.DeviceGrant) error {
	if err := m.fire(GrantOpened); err != nil {
		return err
	}
	expiresAt, _ := time.Parse(time.RFC3339, grant.ExpiresAt)
	m.Grant = GrantInfo{
		UserCode:        grant.UserCode,
		VerificationURL: grant.VerificationURL,
		ExpiresAt:       expiresAt,
	}
	return nil
}

// GrantApproved moves from screen 1 to the prerequisites screen.
func (m *Machine) GrantApproved() error {
	return m.fire(GrantApproved)
}

// GrantExpired returns to screen 0 with the "code expired" banner.
func (m *Machine) GrantExpired() error {
	if err := m.fire(GrantExpired); err != nil {
		return err
	}
	m.ExpiredBanner = true
	return nil
}

// PrereqsOK records a passing prerequisites check (DPS meter found) and
// lands on Ready or UpToDate depending on how many combats are pending —
// the refinement Next's table can't make on its own (state.go explains
// why). It's reached both from Prerequisites ("Terminer") and from Blocked
// ("Réessayer"/"Choisir le dossier" finding the meter after all).
func (m *Machine) PrereqsOK(dpsMeter DPSMeterInfo) error {
	if err := m.fire(PrereqsOK); err != nil {
		return err
	}
	m.DPSMeter = dpsMeter
	if dpsMeter.PendingCount > 0 {
		m.Current = Ready
	} else {
		m.Current = UpToDate
	}
	return nil
}

// PrereqsMissing records a failing prerequisites check without leaving the
// current screen (Prerequisites or Blocked).
func (m *Machine) PrereqsMissing(dpsMeter DPSMeterInfo) error {
	if err := m.fire(PrereqsMissing); err != nil {
		return err
	}
	m.DPSMeter = dpsMeter
	return nil
}

// SyncStarted moves to the syncing screen, clearing any progress or
// rate-limit data left over from a previous run.
func (m *Machine) SyncStarted() error {
	if err := m.fire(SyncStarted); err != nil {
		return err
	}
	m.SyncProgress = ProgressInfo{}
	m.RateLimit = RateLimitInfo{}
	return nil
}

// Progress reports how far the current sync run has gotten.
func (m *Machine) Progress(update ProgressInfo) error {
	if err := m.fire(Progress); err != nil {
		return err
	}
	m.SyncProgress = update
	return nil
}

// RateLimited records a 429 the running sync just hit. now is the moment
// it happened — kept separate from time.Now() so tests can pick any value
// and BuildViewModel's countdown stays reproducible.
func (m *Machine) RateLimited(retryAfter time.Duration, now time.Time) error {
	if err := m.fire(RateLimited); err != nil {
		return err
	}
	m.RateLimit = RateLimitInfo{Active: true, RetryAfter: retryAfter, RetryAt: now.Add(retryAfter)}
	return nil
}

// Resumed clears the rate-limit banner once the retry delay has elapsed and
// uploading continues.
func (m *Machine) Resumed() error {
	if err := m.fire(Resumed); err != nil {
		return err
	}
	m.RateLimit = RateLimitInfo{}
	return nil
}

// SyncFinished moves to the Done screen with the run's summary.
// remainingPending is the pending count right after this run (normally 0
// for a clean full sync) — stored on DPSMeter so a later DeriveIdleState
// call (cmd/gui's "Fermer" handler) picks the right idle screen without
// re-scanning the disk itself.
func (m *Machine) SyncFinished(summary sync.Summary, remainingPending int) error {
	if err := m.fire(SyncFinished); err != nil {
		return err
	}
	m.Summary = newSummaryInfo(summary)
	m.DPSMeter.PendingCount = remainingPending
	return nil
}

// SyncFailed moves to Blocked for an ordinary failure (network down, the
// history folder vanished mid-run), or back to Unlinked with the revoked
// banner when the server rejected the credential (401, F17) — the
// refinement Next's table can't make on its own.
func (m *Machine) SyncFailed(cause error) error {
	if err := m.fire(SyncFailed); err != nil {
		return err
	}
	if errors.Is(cause, api.ErrUnauthorized) {
		m.Current = Unlinked
		m.RevokedBanner = true
		return nil
	}
	m.Block = BlockInfo{Reason: BlockNetwork, Err: cause}
	return nil
}

// UnlinkRequested forgets everything and returns to screen 0 — "Annuler"
// from Linking, or "Délier ce PC" from Settings/Done/Blocked. The reset is
// deliberate: an unlink always implies the credential (and so every
// account-scoped field: grant, character, summary, block reason) is gone.
func (m *Machine) UnlinkRequested() error {
	if err := m.fire(UnlinkRequested); err != nil {
		return err
	}
	*m = Machine{Current: Unlinked}
	return nil
}

// SettingsOpened opens screen 8, remembering the screen to return to.
func (m *Machine) SettingsOpened() error {
	previous := m.Current
	if err := m.fire(SettingsOpened); err != nil {
		return err
	}
	m.Previous = previous
	return nil
}

// SettingsClosed returns to whatever screen was showing before Settings
// was opened. Next's table has a placeholder target for {Settings,
// SettingsClosed} (state.go explains why); this overrides it with Previous.
func (m *Machine) SettingsClosed() error {
	if _, ok := Next(m.Current, SettingsClosed); !ok {
		return fmt.Errorf("%s in %s: %w", SettingsClosed, m.Current, ErrInvalidTransition)
	}
	m.Current = m.Previous
	return nil
}

// DeriveIdleState picks the screen a fresh launch, or a closed Done screen,
// should land on — purely from what's true on disk right now:
//
//	no credential               -> Unlinked (screen 0)
//	credential, meter not found -> Blocked (screen 7)
//	credential, meter found     -> Ready (screen 3) if something is
//	                               pending, UpToDate (screen 6) otherwise
func DeriveIdleState(hasCredential, dpsMeterFound bool, pendingCount int) State {
	if !hasCredential {
		return Unlinked
	}
	if !dpsMeterFound {
		return Blocked
	}
	if pendingCount > 0 {
		return Ready
	}
	return UpToDate
}

// newSummaryInfo reduces a sync.Summary's raw server reason codes to
// screen 5's three tiles, so nothing above this package needs to know the
// codes exist.
func newSummaryInfo(s sync.Summary) SummaryInfo {
	info := SummaryInfo{
		Recorded:        s.Recorded,
		AlreadyRecorded: s.AlreadyRecorded,
		Failed:          s.Failed,
	}
	for reason, n := range s.Refused {
		switch reason {
		case "NO_MATCHING_CHARACTER", "AMBIGUOUS_MATCH":
			info.NoMatchingCharacter += n
		default:
			info.Unreadable += n
		}
	}
	for _, n := range s.Skipped {
		info.Unreadable += n
	}
	return info
}
