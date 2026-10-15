package main

import (
	"testing"
	"time"
)

func TestDownloadProgress_percentIsMinusOneWithoutATotal(t *testing.T) {
	p := downloadProgress{Downloaded: 1024, Total: 0}
	if got := p.percent(); got != -1 {
		t.Fatalf("percent = %d, want -1", got)
	}
}

func TestDownloadProgress_percent(t *testing.T) {
	p := downloadProgress{Downloaded: 25, Total: 100}
	if got := p.percent(); got != 25 {
		t.Fatalf("percent = %d, want 25", got)
	}
}

func TestDownloadProgress_labelWithoutATotal(t *testing.T) {
	p := downloadProgress{Downloaded: 1024, StartedAt: time.Now()}
	if got := p.label(time.Now()); got != "Téléchargement en cours…" {
		t.Fatalf("label = %q", got)
	}
}

func TestDownloadProgress_labelBeforeAnyRateIsKnown(t *testing.T) {
	now := time.Now()
	p := downloadProgress{Downloaded: 0, Total: 1000, StartedAt: now}
	if got := p.label(now); got != "Téléchargement… 0 %" {
		t.Fatalf("label = %q", got)
	}
}

func TestDownloadProgress_labelWithAnETA(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	p := downloadProgress{
		Downloaded: 50,
		Total:      100,
		StartedAt:  now.Add(-10 * time.Second),
	}
	// 50 bytes in 10s -> 5 B/s; 50 remaining -> 10s ETA.
	want := "Téléchargement… 50 % · encore environ 10 s"
	if got := p.label(now); got != want {
		t.Fatalf("label = %q, want %q", got, want)
	}
}

func TestDownloadProgress_labelWithAnETAInMinutes(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	p := downloadProgress{
		Downloaded: 1,
		Total:      121,
		StartedAt:  now.Add(-1 * time.Second),
	}
	// 1 byte/s; 120 remaining -> 120s = 2min ETA.
	want := "Téléchargement… 0 % · encore environ 2 min"
	if got := p.label(now); got != want {
		t.Fatalf("label = %q, want %q", got, want)
	}
}

func TestEtaLabel(t *testing.T) {
	cases := map[int]string{0: "0 s", 30: "30 s", 59: "59 s", 60: "1 min", 125: "2 min"}
	for seconds, want := range cases {
		if got := etaLabel(seconds); got != want {
			t.Fatalf("etaLabel(%d) = %q, want %q", seconds, got, want)
		}
	}
}
