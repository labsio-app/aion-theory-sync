// Command aion-theory-sync links this machine to a member's Aion Theory
// account (device flow) and uploads the AION 2 combat logs the A2Tools DPS
// meter has written, so their measures attach to the member's declared
// character (US-COMBAT-001).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/labsio-app/aion-theory-sync/internal/api"
	"github.com/labsio-app/aion-theory-sync/internal/auth"
	"github.com/labsio-app/aion-theory-sync/internal/sync"
)

const defaultServerURL = "https://aion-theory.labsio.app"

func main() {
	server := flag.String("server", defaultServerURL, "Aion Theory server base URL")
	historyDir := flag.String("history-dir", sync.HistoryDir(), "A2Tools DPS meter history folder")
	configDir := flag.String("config-dir", auth.ConfigDir(), "where this tool keeps its own credential")
	relink := flag.Bool("relink", false, "forget the stored credential and link this device again")
	flag.Parse()

	if err := run(*server, *historyDir, *configDir, *relink, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(serverURL, historyDir, configDir string, relink bool, out *os.File) error {
	if historyDir == "" {
		return errors.New("no A2Tools DPS meter history folder found; pass -history-dir")
	}

	ctx := context.Background()
	client := api.New(serverURL)

	if relink {
		if err := auth.ClearCredential(configDir); err != nil {
			return fmt.Errorf("forgetting the stored credential: %w", err)
		}
	}

	credential, err := auth.EnsureCredential(ctx, client, configDir, out)
	if err != nil {
		return fmt.Errorf("linking this device: %w", err)
	}

	summary, err := sync.Run(ctx, client, credential, historyDir, out)
	if errors.Is(err, api.ErrUnauthorized) {
		// The server no longer honors this credential (revoked, or
		// expired past its sliding renewal window, F16): forget it and
		// link again rather than failing the run outright.
		if err := auth.ClearCredential(configDir); err != nil {
			return fmt.Errorf("forgetting a rejected credential: %w", err)
		}
		credential, err = auth.RunDeviceFlow(ctx, client, configDir, out)
		if err != nil {
			return fmt.Errorf("relinking this device: %w", err)
		}
		summary, err = sync.Run(ctx, client, credential, historyDir, out)
	}
	if err != nil {
		return fmt.Errorf("syncing combat logs: %w", err)
	}

	fmt.Fprintln(out, summary.String())
	return nil
}
