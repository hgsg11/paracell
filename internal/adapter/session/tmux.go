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

const (
	paracellClockFormat        = "%H:%M %d-%b-%y"
	paracellDefaultStatusRight = "#{?window_bigger,[#{window_offset_x}#,#{window_offset_y}] ,}" + paracellClockFormat
)

func (a TmuxAdapter) CreateSession(ctx context.Context, cell domain.Cell) error {
	if len(cell.Session.Windows) == 0 {
		if err := a.Runner.Run(ctx, "tmux", "new-session", "-d", "-s", cell.Session.Name, "-e", "PARACELL_CELL="+cell.Name, "-e", "PARACELL_ROOT="+a.Root, "-c", cell.Source.Path); err != nil {
			return err
		}
		return a.configureCellSession(ctx, cell)
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
	return a.configureCellSession(ctx, cell)
}

func (a TmuxAdapter) runWindowCommand(ctx context.Context, cell domain.Cell, window domain.SessionWindow) error {
	if window.Command == "" {
		return nil
	}
	return a.Runner.Run(ctx, "tmux", "send-keys", "-t", cell.Session.Name+":"+window.Name, window.Command, "Enter")
}

func (a TmuxAdapter) configureCellSession(ctx context.Context, cell domain.Cell) error {
	if err := a.Runner.Run(ctx, "tmux", "set-environment", "-t", cell.Session.Name, "PARACELL_CELL", cell.Name); err != nil {
		return err
	}
	if err := a.Runner.Run(ctx, "tmux", "set-environment", "-t", cell.Session.Name, "PARACELL_ROOT", a.Root); err != nil {
		return err
	}
	windowTargets := make([]string, 0, len(cell.Session.Windows))
	for _, window := range cell.Session.Windows {
		windowTargets = append(windowTargets, cell.Session.Name+":"+window.Name)
	}
	if len(windowTargets) == 0 {
		windowTargets = append(windowTargets, cell.Session.Name)
	}
	return a.configureSession(ctx, cell.Session.Name, cellProjectName(cell), cell.Name, windowTargets)
}

func (a TmuxAdapter) configureSession(ctx context.Context, target string, project string, label string, windowTargets []string) error {
	if err := a.Runner.Run(ctx, "tmux", "set-option", "-t", target, "@paracell-project", project); err != nil {
		return err
	}
	if err := a.Runner.Run(ctx, "tmux", "set-option", "-t", target, "@paracell-status-label", label); err != nil {
		return err
	}
	if err := a.Runner.Run(ctx, "tmux", "set-option", "-t", target, "set-titles", "on"); err != nil {
		return err
	}
	if err := a.Runner.Run(ctx, "tmux", "set-option", "-t", target, "set-titles-string", "#{@paracell-project}"); err != nil {
		return err
	}
	if err := a.Runner.Run(ctx, "tmux", "set-option", "-t", target, "status-left", "#{@paracell-status-label} "); err != nil {
		return err
	}
	if err := a.Runner.Run(ctx, "tmux", "set-option", "-t", target, "status-left-length", "100"); err != nil {
		return err
	}
	statusRight, err := a.Runner.Output(ctx, "tmux", "show-option", "-v", "-t", target, "status-right")
	if err != nil {
		return err
	}
	statusRight = strings.TrimSuffix(statusRight, "\n")
	if !strings.Contains(statusRight, paracellClockFormat) {
		if statusRight == "" {
			statusRight = paracellDefaultStatusRight
		} else {
			statusRight += " " + paracellClockFormat
		}
		if err := a.Runner.Run(ctx, "tmux", "set-option", "-t", target, "status-right", statusRight); err != nil {
			return err
		}
	}
	windowFormat := "#{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }"
	if listed, err := a.Runner.Output(ctx, "tmux", "list-windows", "-t", target, "-F", "#{window_id}"); err == nil && strings.TrimSpace(listed) != "" {
		windowTargets = strings.Fields(listed)
	}
	for _, windowTarget := range windowTargets {
		if err := a.Runner.Run(ctx, "tmux", "set-window-option", "-t", windowTarget, "window-status-format", windowFormat); err != nil {
			return err
		}
		if err := a.Runner.Run(ctx, "tmux", "set-window-option", "-t", windowTarget, "window-status-current-format", windowFormat); err != nil {
			return err
		}
	}
	newWindowHook := "set-window-option window-status-format '" + windowFormat + "'; set-window-option window-status-current-format '" + windowFormat + "'"
	if err := a.Runner.Run(ctx, "tmux", "set-hook", "-t", target, "after-new-window[100]", newWindowHook); err != nil {
		return err
	}
	if err := a.Runner.Run(ctx, "tmux", "set-option", "-t", target, "key-table", "paracell"); err != nil {
		return err
	}
	if err := a.Runner.Run(ctx, "tmux", "set-option", "-t", target, "mouse", "on"); err != nil {
		return err
	}
	if err := a.Runner.Run(ctx, "tmux", "set-option", "-t", target, "set-clipboard", "on"); err != nil {
		return err
	}
	if err := a.Runner.Run(ctx, "tmux", "bind-key", "-T", "paracell", "MouseDown1Pane", "select-pane", "-t", "=", "\\;", "send-keys", "-M"); err != nil {
		return err
	}
	if err := a.Runner.Run(ctx, "tmux", "bind-key", "-T", "paracell", "MouseDrag1Pane", "if-shell", "-F", "#{||:#{pane_in_mode},#{mouse_any_flag}}", "send-keys -M", "copy-mode -M"); err != nil {
		return err
	}
	if err := a.Runner.Run(ctx, "tmux", "bind-key", "-T", "paracell", "WheelUpPane", "if-shell", "-F", "#{||:#{alternate_on},#{pane_in_mode},#{mouse_any_flag}}", "send-keys -M", "copy-mode -e"); err != nil {
		return err
	}
	if err := a.Runner.Run(ctx, "tmux", "bind-key", "-T", "paracell", "C-t", "next-window"); err != nil {
		return err
	}
	args := []string{"bind-key", "-T", "paracell", "C-p", "display-popup", "-w", "65", "-h", "50%"}
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
	if err := a.PrepareSession(ctx, cell); err != nil {
		return err
	}
	if os.Getenv("TMUX") != "" {
		return a.Runner.Run(ctx, "tmux", "switch-client", "-E", "-t", cell.Session.Name)
	}
	return a.Runner.Run(ctx, "tmux", "attach-session", "-E", "-t", cell.Session.Name)
}

func (a TmuxAdapter) PrepareSession(ctx context.Context, cell domain.Cell) error {
	return a.configureCellSession(ctx, cell)
}

func (a TmuxAdapter) EnterRootSession(ctx context.Context, project domain.ProjectConfig) error {
	name := rootSessionName(project.Name)
	if err := a.ensureRootSession(ctx, name); err != nil {
		return err
	}
	if os.Getenv("TMUX") != "" {
		return a.Runner.Run(ctx, "tmux", "switch-client", "-E", "-t", name)
	}
	return a.Runner.Run(ctx, "tmux", "attach-session", "-E", "-t", name)
}

func (a TmuxAdapter) ExitSession(ctx context.Context) error {
	if os.Getenv("TMUX") == "" {
		return errors.New("paracell exit must be run inside tmux")
	}
	return a.Runner.Run(ctx, "tmux", "detach-client")
}

func (a TmuxAdapter) ensureRootSession(ctx context.Context, name string) error {
	err := a.Runner.Run(ctx, "tmux", "has-session", "-t", name)
	if err == nil {
		return a.configureRootSession(ctx, name)
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
	return a.configureRootSession(ctx, name)
}

func (a TmuxAdapter) configureRootSession(ctx context.Context, name string) error {
	if err := a.Runner.Run(ctx, "tmux", "set-environment", "-u", "-t", name, "PARACELL_CELL"); err != nil {
		return err
	}
	if err := a.Runner.Run(ctx, "tmux", "set-environment", "-t", name, "PARACELL_ROOT", a.Root); err != nil {
		return err
	}
	return a.configureSession(ctx, name, strings.TrimSuffix(name, "-root"), "root", []string{name})
}

func cellProjectName(cell domain.Cell) string {
	project := strings.TrimSuffix(cell.Session.Name, "-"+cell.Name)
	if project == "" || project == cell.Session.Name {
		return cell.Session.Name
	}
	return project
}

func rootSessionName(project string) string {
	return project + "-root"
}
