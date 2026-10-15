package state

import (
	"fmt"
	"time"
)

// Plural picks singular or plural depending on n, following the one rule
// the UX spec actually calls out ("singulier si 1") rather than French's
// full agreement rules — every call site needs exactly two wordings.
func Plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// RelativeDate renders t relative to now, in local time, the way the "Prêt
// à synchroniser" screen's "Dernière synchro" line does. The UX spec gives
// three example buckets (just now/minutes, yesterday, an older date) but
// not a boundary for a same-day sync more than an hour old — that fourth
// bucket ("aujourd'hui à HH:MM") is this package's own, documented, choice
// rather than leaving a multi-hour-old sync stuck reading "il y a 180 min".
func RelativeDate(t, now time.Time) string {
	t, now = t.Local(), now.Local()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "à l'instant"
	case diff < time.Hour:
		return fmt.Sprintf("il y a %d min", int(diff/time.Minute))
	}

	if sameDay(t, now) {
		return "aujourd'hui à " + t.Format("15:04")
	}
	if sameDay(t, now.AddDate(0, 0, -1)) {
		return "hier à " + t.Format("15:04")
	}
	return "le " + t.Format("02/01") + " à " + t.Format("15:04")
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// Countdown renders a remaining duration as mm:ss, for the device-code
// expiry timer (screen 1) and the rate-limit retry countdown (screen 4). A
// negative remainder (the deadline has already passed) clamps to 00:00
// rather than printing a negative number.
func Countdown(remaining time.Duration) string {
	if remaining < 0 {
		remaining = 0
	}
	total := int(remaining.Round(time.Second) / time.Second)
	return fmt.Sprintf("%02d:%02d", total/60, total%60)
}

// FightDuration renders a combat's length the way screen 4's "current
// file" line does: "3 min 12 s", or just "45 s" under a minute.
func FightDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Round(time.Second) / time.Second)
	minutes, seconds := total/60, total%60
	if minutes == 0 {
		return fmt.Sprintf("%d s", seconds)
	}
	return fmt.Sprintf("%d min %d s", minutes, seconds)
}
