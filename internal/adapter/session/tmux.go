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

func NewTmuxAdapter(runner system.Runner, root string) TmuxAdapter {
	return TmuxAdapter{Runner: runner, Root: root}
}

const (
	paracellClockFormat        = "%H:%M %d-%b-%y"
	paracellDefaultStatusRight = "#{?window_bigger,[#{window_offset_x}#,#{window_offset_y}] ,}" + paracellClockFormat
)

func (a TmuxAdapter) CreateSession(ctx context.Context, resource domain.SessionResource) (returnErr error) {
	if len(resource.Windows) == 0 {
		if err := a.Runner.Run(ctx, "tmux", "new-session", "-d", "-s", resource.Name, "-e", "PARACELL_CELL="+resource.CellName, "-e", "PARACELL_ROOT="+a.Root, "-c", resource.WorkingDirectory); err != nil {
			return err
		}
		defer a.cleanupFailedCreation(ctx, resource, &returnErr)
		return a.configureCellSession(ctx, resource)
	}
	first := resource.Windows[0]
	if err := a.Runner.Run(ctx, "tmux", "new-session", "-d", "-s", resource.Name, "-e", "PARACELL_CELL="+resource.CellName, "-e", "PARACELL_ROOT="+a.Root, "-n", first.Name, "-c", resource.WorkingDirectory); err != nil {
		return err
	}
	defer a.cleanupFailedCreation(ctx, resource, &returnErr)
	if err := a.runWindowCommand(ctx, resource, first); err != nil {
		return err
	}
	for _, window := range resource.Windows[1:] {
		if err := a.Runner.Run(ctx, "tmux", "new-window", "-t", resource.Name, "-n", window.Name, "-c", resource.WorkingDirectory); err != nil {
			return err
		}
		if err := a.runWindowCommand(ctx, resource, window); err != nil {
			return err
		}
	}
	return a.configureCellSession(ctx, resource)
}

func (a TmuxAdapter) cleanupFailedCreation(ctx context.Context, resource domain.SessionResource, returnErr *error) {
	if *returnErr == nil {
		return
	}
	if err := a.CleanSession(context.WithoutCancel(ctx), resource); err != nil && !errors.Is(err, domain.ErrNotFound) {
		*returnErr = errors.Join(*returnErr, fmt.Errorf("clean partial tmux session: %w", err))
	}
}

func (a TmuxAdapter) runWindowCommand(ctx context.Context, resource domain.SessionResource, window domain.SessionWindow) error {
	if window.Command == "" {
		return nil
	}
	return a.Runner.Run(ctx, "tmux", "send-keys", "-t", resource.Name+":"+window.Name, window.Command, "Enter")
}

func (a TmuxAdapter) configureCellSession(ctx context.Context, resource domain.SessionResource) error {
	if err := a.Runner.Run(ctx, "tmux", "set-environment", "-t", resource.Name, "PARACELL_CELL", resource.CellName); err != nil {
		return err
	}
	if err := a.Runner.Run(ctx, "tmux", "set-environment", "-t", resource.Name, "PARACELL_ROOT", a.Root); err != nil {
		return err
	}
	windowTargets := make([]string, 0, len(resource.Windows))
	for _, window := range resource.Windows {
		windowTargets = append(windowTargets, resource.Name+":"+window.Name)
	}
	if len(windowTargets) == 0 {
		windowTargets = append(windowTargets, resource.Name)
	}
	return a.configureSession(ctx, resource.Name, resource.Project, resource.DisplayLabel, windowTargets)
}

func (a TmuxAdapter) UpdateStatusLabel(ctx context.Context, resource domain.SessionResource) error {
	err := a.Runner.Run(ctx, "tmux", "set-option", "-t", resource.Name, "@paracell-status-label", resource.DisplayLabel)
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "can't find session") {
		return fmt.Errorf("%w: %v", domain.ErrNotFound, err)
	}
	return err
}

func (a TmuxAdapter) configureSession(ctx context.Context, target string, project string, label string, windowTargets []string) error {
	keyTable := "paracell-" + target
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
	if err := a.Runner.Run(ctx, "tmux", "set-option", "-t", target, "key-table", keyTable); err != nil {
		return err
	}
	if err := a.Runner.Run(ctx, "tmux", "set-option", "-t", target, "mouse", "on"); err != nil {
		return err
	}
	if err := a.Runner.Run(ctx, "tmux", "set-option", "-t", target, "set-clipboard", "on"); err != nil {
		return err
	}
	if err := a.Runner.Run(ctx, "tmux", "bind-key", "-T", keyTable, "MouseDown1Pane", "select-pane", "-t", "=", "\\;", "send-keys", "-M"); err != nil {
		return err
	}
	if err := a.Runner.Run(ctx, "tmux", "bind-key", "-T", keyTable, "MouseDrag1Pane", "if-shell", "-F", "#{||:#{pane_in_mode},#{mouse_any_flag}}", "send-keys -M", "copy-mode -M"); err != nil {
		return err
	}
	if err := a.Runner.Run(ctx, "tmux", "bind-key", "-T", keyTable, "WheelUpPane", "if-shell", "-F", "#{||:#{alternate_on},#{pane_in_mode},#{mouse_any_flag}}", "send-keys -M", "copy-mode -e"); err != nil {
		return err
	}
	if err := a.Runner.Run(ctx, "tmux", "bind-key", "-T", keyTable, "C-t", "next-window"); err != nil {
		return err
	}
	args := []string{"bind-key", "-T", keyTable, "C-p", "display-popup", "-t", target, "-w", "65", "-h", "24"}
	if a.Root != "" {
		args = append(args, "-d", a.Root)
	}
	args = append(args, "-E", "paracell", "view")
	return a.Runner.Run(ctx, "tmux", args...)
}

func (a TmuxAdapter) CleanSession(ctx context.Context, resource domain.SessionResource) error {
	err := a.Runner.Run(ctx, "tmux", "kill-session", "-t", resource.Name)
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "can't find session") {
		return fmt.Errorf("%w: %v", domain.ErrNotFound, err)
	}
	return err
}

func (a TmuxAdapter) EnterSession(ctx context.Context, resource domain.SessionResource) error {
	if err := a.PrepareSession(ctx, resource); err != nil {
		return err
	}
	if os.Getenv("TMUX") != "" {
		return a.Runner.Run(ctx, "tmux", "switch-client", "-E", "-t", resource.Name)
	}
	return a.Runner.Run(ctx, "tmux", "attach-session", "-E", "-t", resource.Name)
}

func (a TmuxAdapter) PrepareSession(ctx context.Context, resource domain.SessionResource) error {
	return a.configureCellSession(ctx, resource)
}

func (a TmuxAdapter) EnterRootSession(ctx context.Context, projectName string) error {
	name := rootSessionName(projectName)
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

func rootSessionName(project string) string {
	return project + "-root"
}
