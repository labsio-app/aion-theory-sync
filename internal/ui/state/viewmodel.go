package state

import (
	"fmt"
	"strings"
	"time"
)

// Every string below is copied verbatim from the UX spec ("Les textes sont
// exacts"), except the two call-outs in BuildViewModel's doc comment: the
// 401-revoked banner and the network-failure screen 7, for which the spec
// explicitly left the wording up to this package ("un bandeau explicatif",
// "un message clair") rather than spelling it out.

// StepperStatus is one of gabarit A's three vertical steps' states.
type StepperStatus string

const (
	StepUpcoming StepperStatus = "upcoming"
	StepCurrent  StepperStatus = "current"
	StepDone     StepperStatus = "done"
)

type StepperStep struct {
	Label  string
	Status StepperStatus
}

func stepper(current int) []StepperStep {
	labels := [3]string{"Lier ton compte", "Vérifier les pré-requis", "Synchroniser"}
	steps := make([]StepperStep, 3)
	for i, label := range labels {
		status := StepUpcoming
		switch {
		case i < current:
			status = StepDone
		case i == current:
			status = StepCurrent
		}
		steps[i] = StepperStep{Label: label, Status: status}
	}
	return steps
}

type UnlinkedView struct {
	Title             string
	Body              string
	PrimaryButton     string
	Hint              string
	ExpiredBanner     bool
	ExpiredBannerText string
	RevokedBanner     bool
	RevokedBannerText string
}

type LinkingView struct {
	Title           string
	Subtitle        string
	UserCode        string
	CopyButton      string
	Expired         bool
	Countdown       string // "mm:ss"
	WaitingText     string
	ReopenButton    string
	VerificationURL string
	CancelText      string
}

type PrereqCard struct {
	Title    string
	Subtitle string
	Done     bool
	// Action is the one link/button shown on the right when the card isn't
	// done yet (empty when Done is true).
	ActionLabel string
	ActionURL   string // "" for an action that isn't a plain link (e.g. "Choisir le dossier")
}

type PrerequisitesView struct {
	Title                string
	AccountCard          PrereqCard
	DPSMeterCard         PrereqCard
	CharacterCard        PrereqCard
	LaunchAtStartupLabel string
	AutoSyncLabel        string
	FinishButton         string
	FinishEnabled        bool
}

type ReadyView struct {
	LastSyncedLabel string
	PendingCount    int
	PendingLabel    string
	SyncNowButton   string
}

type SyncingView struct {
	Done, Total       int
	StatusText        string
	CurrentFileLabel  string
	RateLimitActive   bool
	RateLimitText     string
	PauseResumeButton string
}

type SummaryTile struct {
	Label   string
	Count   int
	Warning bool // orange border, per the spec, when Count > 0
}

type DoneView struct {
	Title               string
	SubtitleVisible     bool
	Subtitle            string
	AlreadyPresent      SummaryTile
	NoMatchingCharacter SummaryTile
	Unreadable          SummaryTile
	FollowupVisible     bool
	FollowupText        string
	FailedBannerVisible bool
	FailedBannerText    string
	CloseButton         string
	ViewOnlineButton    string
}

type UpToDateView struct {
	LastSyncedLabel string
	Title           string
	Body            string
	SyncNowButton   string
	SyncNowDisabled bool
}

type BlockedView struct {
	Title            string
	Body             string
	PrimaryButton    string // "" when this block reason has none (network)
	RetryButton      string
	InstallLinkText  string // "" when this block reason has none (network)
	TechnicalLogLink string
}

// ViewModel is everything one of cmd/gui's screens needs to render,
// derived purely from a Machine snapshot and the current time — no field
// here requires cmd/gui to branch on business logic, only to lay widgets
// out and bind them to these values.
type ViewModel struct {
	Screen  State
	Stepper []StepperStep // populated only for the three gabarit A screens

	// CharacterKnown/CharacterSummary mirror Machine.Character, formatted
	// once here so gabarit B's sidebar (shown next to every screen below,
	// not just one) never has to format a character list itself.
	CharacterKnown   bool
	CharacterSummary string

	Unlinked      UnlinkedView
	Linking       LinkingView
	Prerequisites PrerequisitesView
	Ready         ReadyView
	Syncing       SyncingView
	Done          DoneView
	UpToDate      UpToDateView
	Blocked       BlockedView
}

// BuildViewModel renders m's current screen. now drives every live
// countdown (grant expiry, rate-limit retry) so the result stays a pure
// function of its inputs instead of reading the wall clock itself.
func BuildViewModel(m *Machine, now time.Time) ViewModel {
	vm := ViewModel{
		Screen:           m.Current,
		CharacterKnown:   m.Character.Known,
		CharacterSummary: characterSummary(m.Character.Characters),
	}

	switch m.Current {
	case Unlinked:
		vm.Stepper = stepper(0)
		vm.Unlinked = buildUnlinkedView(m)
	case Linking:
		vm.Stepper = stepper(0)
		vm.Linking = buildLinkingView(m, now)
	case Prerequisites:
		vm.Stepper = stepper(1)
		vm.Prerequisites = buildPrerequisitesView(m)
	case Ready:
		vm.Ready = buildReadyView(m, now)
	case Syncing:
		vm.Syncing = buildSyncingView(m, now)
	case Done:
		vm.Done = buildDoneView(m)
	case UpToDate:
		vm.UpToDate = buildUpToDateView(m, now)
	case Blocked:
		vm.Blocked = buildBlockedView(m)
	}
	return vm
}

func buildUnlinkedView(m *Machine) UnlinkedView {
	return UnlinkedView{
		Title: "Lie cette application à ton compte Aion Theory",
		Body: "On te donne un code, tu le valides sur Aion Theory depuis ton navigateur. " +
			"Ton mot de passe ne passe jamais par l'application, et tu peux révoquer le lien à tout moment depuis ton compte.",
		PrimaryButton:     "Lier mon compte",
		Hint:              "Il te faut un compte Aion Theory existant.",
		ExpiredBanner:     m.ExpiredBanner,
		ExpiredBannerText: "Le code a expiré. Relance la liaison.",
		RevokedBanner:     m.RevokedBanner,
		// Not spec-literal: the UX spec calls only for "un bandeau
		// explicatif" here without giving its wording (unlike every other
		// screen's copy, which is quoted exactly).
		RevokedBannerText: "Le lien avec ton compte a été révoqué. Relie à nouveau cet appareil.",
	}
}

func buildLinkingView(m *Machine, now time.Time) LinkingView {
	remaining := m.Grant.ExpiresAt.Sub(now)
	countdown := Countdown(remaining)
	return LinkingView{
		Title:           "Valide ce code sur Aion Theory",
		Subtitle:        "Ton navigateur vient de s'ouvrir. Connecte-toi, puis saisis le code.",
		UserCode:        m.Grant.UserCode,
		CopyButton:      "Copier",
		Expired:         remaining <= 0,
		Countdown:       countdown,
		WaitingText:     "En attente de ta validation · le code expire dans " + countdown,
		ReopenButton:    "Rouvrir Aion Theory ↗",
		VerificationURL: m.Grant.VerificationURL,
		CancelText:      "Annuler",
	}
}

func buildPrerequisitesView(m *Machine) PrerequisitesView {
	accountCard := PrereqCard{
		Title: "Compte Aion Theory",
		// "[pseudo]" from the spec needs a member-scoped endpoint this
		// client has no way to call yet (same gap as CharacterInfo) - "Lié"
		// on its own degrades gracefully instead of guessing a name.
		Subtitle: "Lié",
		Done:     true,
	}

	meterCard := PrereqCard{Title: "DPS Meter A2Tools"}
	if m.DPSMeter.Found {
		meterCard.Done = true
		meterCard.Subtitle = fmt.Sprintf("Installé · %d %s trouvé%s",
			m.DPSMeter.PendingCount,
			Plural(m.DPSMeter.PendingCount, "combat", "combats"),
			Plural(m.DPSMeter.PendingCount, "", "s"),
		)
	} else {
		meterCard.Subtitle = "Introuvable"
		meterCard.ActionLabel = "Installer ↗"
		meterCard.ActionURL = "https://a2tools.app"
	}

	characterCard := PrereqCard{Title: "Personnage déclaré"}
	if m.Character.Known {
		characterCard.Done = true
		characterCard.Subtitle = characterSummary(m.Character.Characters)
	} else {
		// Not spec-literal: the server endpoint only shipped after this
		// screen's copy was written, so the empty-state wording and its
		// action (cmd/gui opens the member's Aion Theory account, see
		// screens.go) are this package's own choice, not quoted from the
		// UX spec.
		characterCard.Subtitle = "Aucun personnage déclaré"
		characterCard.ActionLabel = "Déclarer un personnage ↗"
	}

	return PrerequisitesView{
		Title:                "Pré-requis",
		AccountCard:          accountCard,
		DPSMeterCard:         meterCard,
		CharacterCard:        characterCard,
		LaunchAtStartupLabel: "Lancer au démarrage de Windows",
		AutoSyncLabel:        "Synchroniser automatiquement les nouveaux combats",
		FinishButton:         "Terminer",
		FinishEnabled:        m.DPSMeter.Found,
	}
}

func buildReadyView(m *Machine, now time.Time) ReadyView {
	return ReadyView{
		LastSyncedLabel: lastSyncedLabel(m.LastSyncedAt, now),
		PendingCount:    m.DPSMeter.PendingCount,
		PendingLabel: Plural(m.DPSMeter.PendingCount,
			"nouveau combat prêt à être envoyé",
			"nouveaux combats prêts à être envoyés"),
		SyncNowButton: "Synchroniser maintenant",
	}
}

func buildSyncingView(m *Machine, now time.Time) SyncingView {
	view := SyncingView{
		Done:             m.SyncProgress.Done,
		Total:            m.SyncProgress.Total,
		StatusText:       "Envoi en cours…",
		CurrentFileLabel: currentFileLabel(m.SyncProgress.Current),
	}
	if m.Paused {
		view.PauseResumeButton = "Reprendre"
	} else {
		view.PauseResumeButton = "Mettre en pause"
	}
	if m.RateLimit.Active {
		remaining := m.RateLimit.RetryAt.Sub(now)
		if remaining < 0 {
			remaining = 0
		}
		seconds := int(remaining.Round(time.Second) / time.Second)
		view.RateLimitActive = true
		view.RateLimitText = fmt.Sprintf("Le serveur demande une courte pause. Reprise automatique dans %d s.", seconds)
	}
	return view
}

func currentFileLabel(f CurrentFile) string {
	if f.StartedAt.IsZero() {
		return ""
	}
	local := f.StartedAt.Local()
	return fmt.Sprintf("Combat du %s à %s · %s",
		frenchShortDate(local), local.Format("15:04"), FightDuration(f.Duration))
}

var frenchMonthAbbrev = [...]string{
	"janv.", "févr.", "mars", "avr.", "mai", "juin",
	"juil.", "août", "sept.", "oct.", "nov.", "déc.",
}

func frenchShortDate(t time.Time) string {
	return fmt.Sprintf("%d %s", t.Day(), frenchMonthAbbrev[t.Month()-1])
}

func buildDoneView(m *Machine) DoneView {
	s := m.Summary
	view := DoneView{
		Title:               fmt.Sprintf("%d %s", s.Recorded, Plural(s.Recorded, "combat envoyé", "combats envoyés")),
		AlreadyPresent:      SummaryTile{Label: "Déjà présents", Count: s.AlreadyRecorded},
		NoMatchingCharacter: SummaryTile{Label: "Sans personnage reconnu", Count: s.NoMatchingCharacter, Warning: s.NoMatchingCharacter > 0},
		Unreadable:          SummaryTile{Label: "Illisibles, ignorés", Count: s.Unreadable},
		CloseButton:         "Fermer",
		ViewOnlineButton:    "Voir sur Aion Theory ↗",
	}
	if m.Character.Known {
		view.SubtitleVisible = true
		// Spec-literal for the single-character case ("Rattachés à
		// [Nom]"); several declared characters is this package's own
		// extension, joined rather than picking one arbitrarily.
		view.Subtitle = "Rattachés à " + characterSummary(m.Character.Characters)
	}
	if s.NoMatchingCharacter > 0 {
		view.FollowupVisible = true
		view.FollowupText = fmt.Sprintf("%d %s dès que tu déclares le personnage concerné.",
			s.NoMatchingCharacter,
			Plural(s.NoMatchingCharacter,
				"combat sera repris automatiquement",
				"combats seront repris automatiquement"))
	}
	if s.Failed > 0 {
		view.FailedBannerVisible = true
		view.FailedBannerText = fmt.Sprintf("%d %s, %s seront retentés à la prochaine synchro.",
			s.Failed,
			Plural(s.Failed, "combat n'a pas pu partir", "combats n'ont pas pu partir"),
			Plural(s.Failed, "il", "ils"))
	}
	return view
}

func buildUpToDateView(m *Machine, now time.Time) UpToDateView {
	return UpToDateView{
		LastSyncedLabel: lastSyncedLabel(m.LastSyncedAt, now),
		Title:           "Tout est à jour",
		Body:            "Lance un combat avec le DPS meter ouvert : il apparaîtra ici.",
		SyncNowButton:   "Synchroniser maintenant",
		SyncNowDisabled: true,
	}
}

func buildBlockedView(m *Machine) BlockedView {
	switch m.Block.Reason {
	case BlockDPSMeterMissing:
		return BlockedView{
			Title: "On ne trouve pas les combats du DPS Meter A2Tools",
			Body: "Le DPS meter n'est peut-être pas installé, ou son dossier d'historique est ailleurs. " +
				"Indique-le une fois, on s'en souviendra.",
			PrimaryButton:    "Choisir le dossier",
			RetryButton:      "Réessayer",
			InstallLinkText:  "Installer le DPS meter ↗",
			TechnicalLogLink: "Journal technique",
		}
	default:
		// Not spec-literal: the UX spec reuses screen 7's layout for "other
		// blockages" ("réseau injoignable...") with "un message clair et
		// une action unique", without quoting its wording the way every
		// other screen's copy is quoted.
		return BlockedView{
			Title:            "Connexion au serveur impossible",
			Body:             "Vérifie ta connexion internet, puis réessaie.",
			RetryButton:      "Réessayer",
			TechnicalLogLink: "Journal technique",
		}
	}
}

// characterSummary formats a member's declared characters as "Yume · Siel"
// (server omitted when the server couldn't resolve it), joining several
// with ", ". Empty when chars is empty.
func characterSummary(chars []Character) string {
	parts := make([]string, len(chars))
	for i, c := range chars {
		if c.Server != "" {
			parts[i] = c.Name + " · " + c.Server
		} else {
			parts[i] = c.Name
		}
	}
	return strings.Join(parts, ", ")
}

func lastSyncedLabel(lastSyncedAt time.Time, now time.Time) string {
	if lastSyncedAt.IsZero() {
		return ""
	}
	return "Dernière synchro : " + RelativeDate(lastSyncedAt, now)
}
