// Package version holds this build's own version string, so the GUI
// (internal/update) can tell whether a newer release exists without
// shelling out to git or reading any file that may not ship with it.
package version

// Version is set via -ldflags at build time, from the git tag the build
// was cut from (see .goreleaser.yml and the windows-installer job in
// .github/workflows/release.yml) — both inject it as "vMAJOR.MINOR.PATCH",
// matching the tag_name GitHub's releases API reports, so internal/update
// can compare the two directly.
//
// It stays "dev" for any build that skips that step, such as a local
// `go build` or ci.yml's build/vet/test pass — internal/update treats
// "dev" as "never check for updates" rather than nagging those builds.
var Version = "dev"
