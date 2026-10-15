// Package state is the GUI's pure state machine and view-model: which
// screen is showing and what it says, derived without any dependency on
// walk or on Windows — so the whole onboarding/sync/settings flow is
// testable on any platform, and cmd/gui is left with nothing but rendering
// and wiring widget callbacks to the events below.
package state

// State is one of the nine screens the GUI can show. Each state maps to
// exactly one screen in the UX spec (0-8, in this same order).
type State string

const (
	Unlinked      State = "unlinked"      // screen 0
	Linking       State = "linking"       // screen 1
	Prerequisites State = "prerequisites" // screen 2
	Ready         State = "ready"         // screen 3
	Syncing       State = "syncing"       // screen 4
	Done          State = "done"          // screen 5
	UpToDate      State = "up_to_date"    // screen 6
	Blocked       State = "blocked"       // screen 7
	Settings      State = "settings"      // screen 8
)

// Event is one thing that happened: a click, a timer, a server response.
type Event string

const (
	LinkRequested   Event = "link_requested"
	GrantOpened     Event = "grant_opened"
	GrantApproved   Event = "grant_approved"
	GrantExpired    Event = "grant_expired"
	PrereqsOK       Event = "prereqs_ok"
	PrereqsMissing  Event = "prereqs_missing"
	SyncStarted     Event = "sync_started"
	Progress        Event = "progress"
	RateLimited     Event = "rate_limited"
	Resumed         Event = "resumed"
	SyncFinished    Event = "sync_finished"
	SyncFailed      Event = "sync_failed"
	UnlinkRequested Event = "unlink_requested"
	SettingsOpened  Event = "settings_opened"
	SettingsClosed  Event = "settings_closed"
)

type transitionKey struct {
	from  State
	event Event
}

// transitions is the one authoritative table: every (state, event) pair the
// machine accepts, and the state it lands on. Two notes on reading it:
//
//   - PrereqsOK always lands on Ready here; Machine.PrereqsOK (machine.go)
//     immediately refines that to UpToDate when there is nothing pending —
//     a decision that needs the pending count, which this table doesn't
//     carry. Same for SyncFailed: it always lands on Blocked here, and
//     Machine.SyncFailed refines it to Unlinked for a revoked credential.
//   - There is no event here for "close the Done screen" or "finish
//     Prerequisites and open Ready/UpToDate": the UX spec's screens 2 and
//     5 don't name a distinct event for those buttons, so cmd/gui re-derives
//     the idle screen with DeriveIdleState instead of dispatching an event
//     (see its doc comment for why that's the deliberate boundary).
var transitions = map[transitionKey]State{
	{Unlinked, LinkRequested}: Linking,

	{Linking, GrantOpened}:     Linking,
	{Linking, GrantApproved}:   Prerequisites,
	{Linking, GrantExpired}:    Unlinked,
	{Linking, UnlinkRequested}: Unlinked, // "Annuler"

	{Prerequisites, PrereqsOK}:      Ready, // refined to UpToDate by Machine.PrereqsOK
	{Prerequisites, PrereqsMissing}: Prerequisites,

	{Ready, SyncStarted}:    Syncing,
	{Ready, SettingsOpened}: Settings,

	{UpToDate, SyncStarted}:    Syncing, // auto-sync can fire this even while idle on screen 6
	{UpToDate, SettingsOpened}: Settings,

	{Syncing, Progress}:     Syncing,
	{Syncing, RateLimited}:  Syncing,
	{Syncing, Resumed}:      Syncing,
	{Syncing, SyncFinished}: Done,
	{Syncing, SyncFailed}:   Blocked, // refined to Unlinked by Machine.SyncFailed on a revoked credential

	{Done, SettingsOpened}:  Settings,
	{Done, UnlinkRequested}: Unlinked,

	{Blocked, PrereqsOK}:       Ready, // refined to UpToDate by Machine.PrereqsOK
	{Blocked, PrereqsMissing}:  Blocked,
	{Blocked, UnlinkRequested}: Unlinked,
	{Blocked, SettingsOpened}:  Settings,

	{Settings, SettingsClosed}:  Unlinked, // placeholder target; Machine.SettingsClosed overrides it with Previous
	{Settings, UnlinkRequested}: Unlinked,
}

// Next reports the state (from, event) leads to, and whether that pair is
// defined at all. It never mutates anything — Machine (machine.go) is the
// stateful wrapper tests and cmd/gui actually drive.
func Next(from State, event Event) (State, bool) {
	to, ok := transitions[transitionKey{from, event}]
	return to, ok
}
