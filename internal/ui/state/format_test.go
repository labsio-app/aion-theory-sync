package state

import (
	"testing"
	"time"
)

func TestPlural(t *testing.T) {
	if got := Plural(1, "combat", "combats"); got != "combat" {
		t.Fatalf("Plural(1, ...) = %q, want combat", got)
	}
	if got := Plural(0, "combat", "combats"); got != "combats" {
		t.Fatalf("Plural(0, ...) = %q, want combats", got)
	}
	if got := Plural(2, "combat", "combats"); got != "combats" {
		t.Fatalf("Plural(2, ...) = %q, want combats", got)
	}
}

func TestRelativeDate(t *testing.T) {
	// Built in time.Local, not time.UTC: RelativeDate renders clock times
	// in local time (the UX spec's "en fuseau local"), so the expected
	// strings below are derived the same way instead of assuming the test
	// machine's offset is zero.
	now := time.Date(2026, 10, 15, 20, 0, 0, 0, time.Local)
	clock := func(t time.Time) string { return t.Format("15:04") }

	threeHoursAgo := now.Add(-3 * time.Hour)
	yesterdayEvening := now.AddDate(0, 0, -1).Add(-3*time.Hour + 5*time.Minute)
	threeDaysAgo := now.AddDate(0, 0, -3)

	cases := []struct {
		name string
		t    time.Time
		want string
	}{
		{"30 seconds ago", now.Add(-30 * time.Second), "à l'instant"},
		{"2 minutes ago", now.Add(-2 * time.Minute), "il y a 2 min"},
		{"59 minutes ago", now.Add(-59 * time.Minute), "il y a 59 min"},
		{"3 hours ago, same day", threeHoursAgo, "aujourd'hui à " + clock(threeHoursAgo)},
		{"yesterday evening", yesterdayEvening, "hier à " + clock(yesterdayEvening)},
		{"three days ago", threeDaysAgo, "le 12/10 à " + clock(threeDaysAgo)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := RelativeDate(c.t, now); got != c.want {
				t.Fatalf("RelativeDate = %q, want %q", got, c.want)
			}
		})
	}
}

func TestCountdown(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "00:00"},
		{45 * time.Second, "00:45"},
		{90 * time.Second, "01:30"},
		{15 * time.Minute, "15:00"},
		{-5 * time.Second, "00:00"},
	}
	for _, c := range cases {
		if got := Countdown(c.d); got != c.want {
			t.Fatalf("Countdown(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestFightDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{45 * time.Second, "45 s"},
		{3*time.Minute + 12*time.Second, "3 min 12 s"},
		{60 * time.Second, "1 min 0 s"},
		{0, "0 s"},
	}
	for _, c := range cases {
		if got := FightDuration(c.d); got != c.want {
			t.Fatalf("FightDuration(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}
