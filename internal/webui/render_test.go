package webui

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labsio-app/aion-theory-sync/internal/ui/state"
)

func TestBuildPage_everyScreenRendersWithoutError(t *testing.T) {
	for _, f := range Fixtures() {
		t.Run(f.Name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := Render(&buf, f.Page()); err != nil {
				t.Fatalf("Render(%s): %v", f.Name, err)
			}
			html := buf.String()
			if !strings.Contains(html, "<!doctype html>") {
				t.Fatalf("Render(%s): missing doctype", f.Name)
			}
			if !strings.Contains(html, "Aion Theory Sync") {
				t.Fatalf("Render(%s): missing app name", f.Name)
			}
		})
	}
}

func TestBuildPage_gabaritSplitMatchesTheOldWalkScreenWidget(t *testing.T) {
	// Mirrors cmd/gui's old screenWidget switch: screens 0-2 are gabarit A
	// (onboarding sidebar + stepper), everything else is gabarit B (nav +
	// status sidebar).
	cases := []struct {
		screen state.State
		want   string
	}{
		{state.Unlinked, "a"},
		{state.Linking, "a"},
		{state.Prerequisites, "a"},
		{state.Ready, "b"},
		{state.Syncing, "b"},
		{state.Done, "b"},
		{state.UpToDate, "b"},
		{state.Blocked, "b"},
		{state.Settings, "b"},
	}
	for _, c := range cases {
		m := state.NewMachine(c.screen)
		vm := state.BuildViewModel(m, fixtureNow)
		p := BuildPage(vm, baseExtra())
		if p.Gabarit != c.want {
			t.Errorf("BuildPage(%s).Gabarit = %q, want %q", c.screen, p.Gabarit, c.want)
		}
	}
}

func TestBuildPage_unlinkedCarriesTheExactSpecCopy(t *testing.T) {
	m := state.NewMachine(state.Unlinked)
	vm := state.BuildViewModel(m, fixtureNow)
	p := BuildPage(vm, baseExtra())
	if p.Unlinked == nil {
		t.Fatal("Unlinked is nil")
	}
	if p.Unlinked.PrimaryButton != "Lier mon compte" {
		t.Errorf("PrimaryButton = %q, want %q", p.Unlinked.PrimaryButton, "Lier mon compte")
	}
}

func TestBuildPage_syncingMaxValueFallsBackToOneWhenTotalIsZero(t *testing.T) {
	m := state.NewMachine(state.Syncing)
	vm := state.BuildViewModel(m, fixtureNow)
	p := BuildPage(vm, baseExtra())
	if p.Syncing == nil {
		t.Fatal("Syncing is nil")
	}
	if p.Syncing.MaxValue != 1 {
		t.Errorf("MaxValue = %d, want 1 when Total is 0 (an empty progress bar, not a divide-by-zero)", p.Syncing.MaxValue)
	}
}

func TestBuildPage_updateBannerOnlyAppearsWhenAvailable(t *testing.T) {
	m := state.NewMachine(state.Ready)
	vm := state.BuildViewModel(m, fixtureNow)

	extra := baseExtra()
	if p := BuildPage(vm, extra); p.UpdateBanner != nil {
		t.Fatal("UpdateBanner should be nil when Update.Available is false")
	}

	extra.Update = UpdateInfo{Available: true, Version: "0.3.0"}
	p := BuildPage(vm, extra)
	if p.UpdateBanner == nil {
		t.Fatal("UpdateBanner should be set when Update.Available is true")
	}
	if !strings.Contains(p.UpdateBanner.Text, "0.3.0") {
		t.Errorf("UpdateBanner.Text = %q, want it to mention the version", p.UpdateBanner.Text)
	}
}

func TestBuildPage_updateBannerDisablesItsButtonWhileSyncing(t *testing.T) {
	m := state.NewMachine(state.Ready)
	vm := state.BuildViewModel(m, fixtureNow)
	extra := baseExtra()
	extra.Update = UpdateInfo{Available: true, Version: "0.3.0", SyncingNow: true}

	p := BuildPage(vm, extra)
	if !p.UpdateBanner.ButtonDisabled {
		t.Error("ButtonDisabled = false while a sync is running, want true")
	}
}

func TestPreviewHandler_servesEveryFixtureAndRejectsUnknownNames(t *testing.T) {
	h := NewPreviewHandler(Fixtures())

	for _, f := range Fixtures() {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/preview/"+f.Name, nil)
		h.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Errorf("GET /preview/%s = %d, want 200", f.Name, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/preview/does-not-exist", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Errorf("GET /preview/does-not-exist = %d, want 404", rec.Code)
	}
}

func TestServer_rendersWhateverProvideReturnsOnEveryRequest(t *testing.T) {
	calls := 0
	s := NewServer(func() Page {
		calls++
		m := state.NewMachine(state.Ready)
		vm := state.BuildViewModel(m, fixtureNow)
		return BuildPage(vm, baseExtra())
	})

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		s.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("GET / = %d, want 200", rec.Code)
		}
	}
	if calls != 2 {
		t.Errorf("Provide called %d times, want 2 (re-read on every request)", calls)
	}
}
