package state

import "time"

// GrantInfo is the device-grant code shown on screen 1, populated once
// Machine.GrantOpened reports it.
type GrantInfo struct {
	UserCode        string
	VerificationURL string
	ExpiresAt       time.Time
}

// DPSMeterInfo is what prerequisite checking (screen 2) and the idle
// screens (3/6) know about the A2Tools DPS meter: whether its history
// folder was found, and how many combats in it are not yet on the server.
type DPSMeterInfo struct {
	Found        bool
	PendingCount int
}

// CharacterInfo is the member's declared character, shown on screen 2's
// third card and in gabarit B's sidebar. Nothing populates this yet: it
// needs a member-scoped "list my declared characters" endpoint
// (GET /api/sync/characters or equivalent) that the aion-theory server does
// not expose today. Known stays false until it does; BuildViewModel hides
// every field that would need it rather than guess a name.
type CharacterInfo struct {
	Known  bool
	Name   string
	Server string
}

// CurrentFile is the combat log actively uploading, shown under the
// progress ring on screen 4.
type CurrentFile struct {
	StartedAt time.Time
	Duration  time.Duration
}

// ProgressInfo is a sync run's progress, reported through Machine.Progress.
type ProgressInfo struct {
	Total   int
	Done    int
	Current CurrentFile
}

// RateLimitInfo is screen 4's "the server asked for a short pause" banner.
// RetryAt is an absolute time (not just a duration) so BuildViewModel can
// compute a live countdown from whatever "now" it's given, rather than
// Machine having to be ticked itself.
type RateLimitInfo struct {
	Active     bool
	RetryAfter time.Duration
	RetryAt    time.Time
}

// SummaryInfo is screen 5's three tiles plus the failed-upload banner,
// already reduced from sync.Summary's raw reason codes (see newSummaryInfo
// in machine.go) so BuildViewModel never has to know those codes exist.
type SummaryInfo struct {
	Recorded            int
	AlreadyRecorded     int
	NoMatchingCharacter int
	Unreadable          int
	Failed              int
}

// BlockReason names why screen 7 is showing, so BuildViewModel can pick the
// right title, action row and icon for it.
type BlockReason string

const (
	BlockDPSMeterMissing BlockReason = "dps_meter_missing"
	BlockNetwork         BlockReason = "network"
)

// BlockInfo is screen 7's content.
type BlockInfo struct {
	Reason BlockReason
	Err    error
}
