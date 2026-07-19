package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/hgsg11/paracell/internal/adapter/system"
	"github.com/hgsg11/paracell/internal/domain"
)

type TmuxAdapter struct {
	Runner system.Runner
	Root   string
}

func (a TmuxAdapter) CreateSession(ctx context.Context, cell domain.Cell) error {
	if len(cell.Session.Windows) == 0 {
		if err := a.Runner.Run(ctx, "tmux", "new-session", "-d", "-s", cell.Session.Name, "-e", "PARACELL_CELL="+cell.Name, "-e", "PARACELL_ROOT="+a.Root, "-c", cell.Source.Path); err != nil {
			return err
		}
		return a.configureSessionBindings(ctx, cell)
	}
	first := cell.Session.Windows[0]
	if err := a.Runner.Run(ctx, "tmux", "new-session", "-d", "-s", cell.Session.Name, "-e", "PARACELL_CELL="+cell.Name, "-e", "PARACELL_ROOT="+a.Root, "-n", first.Name, "-c", cell.Source.Path); err != nil {
		return err
	}
	if err := a.runWindowCommand(ctx, cell, first); err != nil {
		return err
	}
	for _, window := range cell.Session.Windows[1:] {
		if err := a.Runner.Run(ctx, "tmux", "new-window", "-t", cell.Session.Name, "-n", window.Name, "-c", cell.Source.Path); err != nil {
			return err
		}
		if err := a.runWindowCommand(ctx, cell, window); err != nil {
			return err
		}
	}
	return a.configureSessionBindings(ctx, cell)
}

func (a TmuxAdapter) runWindowCommand(ctx context.Context, cell domain.Cell, window domain.SessionWindow) error {
	if window.Command == "" {
		return nil
	}
	return a.Runner.Run(ctx, "tmux", "send-keys", "-t", cell.Session.Name+":"+window.Name, window.Command, "Enter")
}

func (a TmuxAdapter) configureSessionBindings(ctx context.Context, cell domain.Cell) error {
	return a.configureBindings(ctx, cell.Session.Name)
}

func (a TmuxAdapter) configureBindings(ctx context.Context, target string) error {
	if err := a.Runner.Run(ctx, "tmux", "set-option", "-t", target, "key-table", "paracell"); err != nil {
		return err
	}
	if err := a.Runner.Run(ctx, "tmux", "bind-key", "-T", "paracell", "C-t", "next-window"); err != nil {
		return err
	}
	args := []string{"bind-key", "-T", "paracell", "C-p", "display-popup", "-w", "60%", "-h", "50%"}
	if a.Root != "" {
		args = append(args, "-d", a.Root, "-E", "env", "PARACELL_ROOT="+a.Root, "paracell", "view")
	} else {
		args = append(args, "-E", "paracell", "view")
	}
	return a.Runner.Run(ctx, "tmux", args...)
}

func (a TmuxAdapter) CleanSession(ctx context.Context, cell domain.Cell) error {
	err := a.Runner.Run(ctx, "tmux", "kill-session", "-t", cell.Session.Name)
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "can't find session") {
		return fmt.Errorf("%w: %v", domain.ErrNotFound, err)
	}
	return err
}

func (a TmuxAdapter) EnterSession(ctx context.Context, cell domain.Cell) error {
	if os.Getenv("TMUX") != "" {
		return a.Runner.Run(ctx, "tmux", "switch-client", "-t", cell.Session.Name)
	}
	return a.Runner.Run(ctx, "tmux", "attach-session", "-t", cell.Session.Name)
}

func (a TmuxAdapter) EnterRootSession(ctx context.Context, project domain.ProjectConfig) error {
	name := rootSessionName(project.Name)
	if err := a.ensureRootSession(ctx, name); err != nil {
		return err
	}
	if os.Getenv("TMUX") != "" {
		return a.Runner.Run(ctx, "tmux", "switch-client", "-t", name)
	}
	return a.Runner.Run(ctx, "tmux", "attach-session", "-t", name)
}

func (a TmuxAdapter) ensureRootSession(ctx context.Context, name string) error {
	err := a.Runner.Run(ctx, "tmux", "has-session", "-t", name)
	if err == nil {
		return a.configureBindings(ctx, name)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) && !strings.Contains(strings.ToLower(err.Error()), "can't find session") {
		return err
	}
	args := []string{"new-session", "-d", "-s", name}
	if a.Root != "" {
		args = append(args, "-e", "PARACELL_ROOT="+a.Root, "-c", a.Root)
	} else {
		args = append(args, "-c", ".")
	}
	if err := a.Runner.Run(ctx, "tmux", args...); err != nil {
		return err
	}
	return a.configureBindings(ctx, name)
}

func rootSessionName(project string) string {
	return project + "-root"
}
