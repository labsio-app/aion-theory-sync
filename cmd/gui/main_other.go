//go:build !windows

// The GUI only ships for windows/amd64 (same constraint as the console tool,
// T3): this stub exists only so `go build ./...`/`go vet ./...` still pass
// on the non-Windows CI runner that builds and tests the rest of the module
// natively.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "aion-theory-sync-gui only runs on Windows.")
	os.Exit(1)
}
