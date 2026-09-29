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

func (a TmuxAdapter) CreateSession(ctx context.Context, name string, cellName string, firstWindow string, workingDirectory string) error {
	args := []string{"new-session", "-d", "-s", name, "-e", "PARACELL_CELL=" + cellName, "-e", "PARACELL_ROOT=" + a.Root}
	if firstWindow != "" {
		args = append(args, "-n", firstWindow)
	}
	args = append(args, "-c", workingDirectory)
	return a.Runner.Run(ctx, "tmux", args...)
}

func (a TmuxAdapter) CreateWindow(ctx context.Context, session string, window string, workingDirectory string) error {
	return a.Runner.Run(ctx, "tmux", "new-window", "-t", session, "-n", window, "-c", workingDirectory)
}

func (a TmuxAdapter) SendWindowCommand(ctx context.Context, session string, window string, command string) error {
	return a.Runner.Run(ctx, "tmux", "send-keys", "-t", session+":"+window, command, "Enter")
}

func (a TmuxAdapter) ConfigureSession(ctx context.Context, name string, cellName string, project string, label string, windowNames []string) error {
	if err := a.Runner.Run(ctx, "tmux", "set-environment", "-t", name, "PARACELL_CELL", cellName); err != nil {
		return err
	}
	if err := a.Runner.Run(ctx, "tmux", "set-environment", "-t", name, "PARACELL_ROOT", a.Root); err != nil {
		return err
	}
	windowTargets := make([]string, 0, len(windowNames))
	for _, window := range windowNames {
		windowTargets = append(windowTargets, name+":"+window)
	}
	if len(windowTargets) == 0 {
		windowTargets = append(windowTargets, name)
	}
	return a.configureSession(ctx, name, project, label, windowTargets)
}

func (a TmuxAdapter) UpdateStatusLabel(ctx context.Context, name string, label string) error {
	err := a.Runner.Run(ctx, "tmux", "set-option", "-t", name, "@paracell-status-label", label)
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

func (a TmuxAdapter) CleanSession(ctx context.Context, name string) error {
	err := a.Runner.Run(ctx, "tmux", "kill-session", "-t", name)
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "can't find session") {
		return fmt.Errorf("%w: %v", domain.ErrNotFound, err)
	}
	return err
}

func (a TmuxAdapter) EnterSession(ctx context.Context, name string, cellName string, project string, label string, windowNames []string) error {
	if err := a.PrepareSession(ctx, name, cellName, project, label, windowNames); err != nil {
		return err
	}
	if os.Getenv("TMUX") != "" {
		return a.Runner.Run(ctx, "tmux", "switch-client", "-E", "-t", name)
	}
	return a.Runner.Run(ctx, "tmux", "attach-session", "-E", "-t", name)
}

func (a TmuxAdapter) PrepareSession(ctx context.Context, name string, cellName string, project string, label string, windowNames []string) error {
	return a.ConfigureSession(ctx, name, cellName, project, label, windowNames)
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
