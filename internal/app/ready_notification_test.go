package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hgsg11/paracell/internal/adapter/state"
	"github.com/hgsg11/paracell/internal/domain"
)

func TestReadyTerminalNotifier(t *testing.T) {
	for _, result := range []string{"success", "execution-error", "missing-executable"} {
		t.Run(result, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("PARACELL_CELL", "123")
			t.Setenv("PARACELL_ROOT", "")
			t.Setenv("PATH", dir)
			calls := filepath.Join(dir, "calls")
			t.Setenv("NOTIFICATION_CALLS", calls)
			if result != "missing-executable" {
				script := "#!/bin/sh\nprintf '%s\\n' call \"$@\" >> \"$NOTIFICATION_CALLS\"\n"
				if result == "execution-error" {
					script += "exit 1\n"
				}
				if err := os.WriteFile(filepath.Join(dir, "terminal-notifier"), []byte(script), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			config := "project: {name: myapp}\nproviders: {source: git, workspace: tmux, notifications: terminal-notifier}\ntemplates: {}\n"
			if err := os.WriteFile(filepath.Join(dir, "paracell.yaml"), []byte(config), 0o600); err != nil {
				t.Fatal(err)
			}
			group, err := domain.NewCellGroup("group-cell-1", "123", "myapp", "default", domain.Git, domain.None, domain.TerminalNotifierNotification)
			if err != nil {
				t.Fatal(err)
			}
			cell, err := domain.NewCommanderCell("cell-1", group.ID, domain.NewWorkspace(domain.Tmux, nil))
			if err != nil {
				t.Fatal(err)
			}
			appTestGroups[group.ID] = &group
			// Shell metacharacters must remain part of the single message argument.
			if err := group.SetNote("done; $(echo unsafe)"); err != nil {
				t.Fatal(err)
			}
			store := state.NewSQLiteCellAdapter(filepath.Join(dir, ".paracell", "state.db"))
			if err := store.SaveCells(context.Background(), appTestSet(cell)); err != nil {
				t.Fatal(err)
			}
			err = Run(context.Background(), []string{"ready"}, dir)
			if result == "success" && err != nil {
				t.Fatal(err)
			}
			if result == "execution-error" {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
					t.Fatalf("expected exit status 1, got %v", err)
				}
			}
			if result == "missing-executable" && !errors.Is(err, exec.ErrNotFound) {
				t.Fatalf("expected executable not found, got %v", err)
			}
			if result != "missing-executable" {
				got, err := os.ReadFile(calls)
				if err != nil {
					t.Fatal(err)
				}
				want := "call\n-message\nReady: " + group.Name().Value + "\n"
				if string(got) != want {
					t.Fatalf("calls = %q, want %q", got, want)
				}
			}
		})
	}
}
