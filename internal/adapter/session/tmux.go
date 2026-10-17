package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// defaultServerSocket is the socket name of the default tmux server, where
// root sessions (and cells created before the dedicated server) live.
const defaultServerSocket = "default"

// withSocket prefixes tmux arguments with -L when socket is not the default server ("").
func withSocket(socket string, args []string) []string {
	if socket == "" {
		return args
	}
	return append([]string{"-L", socket}, args...)
}

func (a TmuxAdapter) run(ctx context.Context, socket string, args ...string) error {
	return a.Runner.Run(ctx, "tmux", withSocket(socket, args)...)
}

func (a TmuxAdapter) output(ctx context.Context, socket string, args ...string) (string, error) {
	return a.Runner.Output(ctx, "tmux", withSocket(socket, args)...)
}

// currentServerSocket returns the socket name of the tmux server running this
// process ($TMUX), or "" outside tmux.
func currentServerSocket() string {
	value := os.Getenv("TMUX")
	if value == "" {
		return ""
	}
	path, _, _ := strings.Cut(value, ",")
	return filepath.Base(path)
}

// locateCellSocket returns the socket holding the cell session: the project's
// dedicated server, or "" for a cell created on the default server before the
// dedicated server existed. Unknown sessions resolve to the dedicated server.
func (a TmuxAdapter) locateCellSocket(ctx context.Context, resource domain.WorkspaceResource) string {
	socket := resource.SocketName()
	if err := a.run(ctx, socket, "has-session", "-t", "="+resource.Name); err == nil {
		return socket
	}
	if err := a.run(ctx, "", "has-session", "-t", "="+resource.Name); err == nil {
		return ""
	}
	return socket
}

// switchTo returns the tmux arguments that move this terminal to a session on
// targetSocket ("" is the default server).
func switchTo(targetSocket string, name string) []string {
	currentSocket := currentServerSocket()
	if currentSocket == "" {
		return withSocket(targetSocket, []string{"attach-session", "-E", "-t", name})
	}
	targetName := targetSocket
	if targetName == "" {
		targetName = defaultServerSocket
	}
	if currentSocket == targetName {
		return withSocket(targetSocket, []string{"switch-client", "-E", "-t", name})
	}
	attach := append([]string{"tmux"}, withSocket(targetSocket, []string{"attach-session", "-E", "-t", name})...)
	quoted := make([]string, len(attach))
	for i, arg := range attach {
		quoted[i] = shellQuote(arg)
	}
	return []string{"detach-client", "-E", strings.Join(quoted, " ")}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func windowNameWithoutWorkspaceLabel(windowName string, label string) string {
	if label == "" {
		return windowName
	}
	prefix := label + ":"
	for strings.HasPrefix(windowName, prefix) {
		windowName = strings.TrimPrefix(windowName, prefix)
	}
	return windowName
}

const (
	paracellClockFormat        = "%H:%M %d-%b-%y"
	paracellDefaultStatusRight = "#{?window_bigger,[#{window_offset_x}#,#{window_offset_y}] ,}" + paracellClockFormat
)

func (a TmuxAdapter) CreateWorkspace(ctx context.Context, resource domain.WorkspaceResource) (returnErr error) {
	socket := resource.SocketName()
	if len(resource.Windows) > 0 {
		windows := make([]domain.WorkspaceWindow, len(resource.Windows))
		for i, window := range resource.Windows {
			window.Name = windowNameWithoutWorkspaceLabel(window.Name, resource.DisplayLabel)
			windows[i] = window
		}
		resource.Windows = windows
	}
	if len(resource.Windows) == 0 {
		if err := a.run(ctx, socket, "new-session", "-d", "-s", resource.Name, "-e", "PARACELL_CELL="+resource.CellName, "-e", "PARACELL_ROOT="+a.Root, "-c", resource.WorkingDirectory); err != nil {
			return err
		}
		defer a.cleanupFailedCreation(ctx, socket, resource, &returnErr)
		return a.configureCellWorkspace(ctx, socket, resource)
	}
	first := resource.Windows[0]
	if err := a.run(ctx, socket, "new-session", "-d", "-s", resource.Name, "-e", "PARACELL_CELL="+resource.CellName, "-e", "PARACELL_ROOT="+a.Root, "-n", first.Name, "-c", resource.WorkingDirectory); err != nil {
		return err
	}
	defer a.cleanupFailedCreation(ctx, socket, resource, &returnErr)
	if err := a.runWindowCommand(ctx, socket, resource, first); err != nil {
		return err
	}
	for _, window := range resource.Windows[1:] {
		if err := a.run(ctx, socket, "new-window", "-t", resource.Name, "-n", window.Name, "-c", resource.WorkingDirectory); err != nil {
			return err
		}
		if err := a.runWindowCommand(ctx, socket, resource, window); err != nil {
			return err
		}
	}
	return a.configureCellWorkspace(ctx, socket, resource)
}

func (a TmuxAdapter) cleanupFailedCreation(ctx context.Context, socket string, resource domain.WorkspaceResource, returnErr *error) {
	if *returnErr == nil {
		return
	}
	if err := a.killSession(context.WithoutCancel(ctx), socket, resource); err != nil && !errors.Is(err, domain.ErrNotFound) {
		*returnErr = errors.Join(*returnErr, fmt.Errorf("clean partial tmux session: %w", err))
	}
}

func (a TmuxAdapter) runWindowCommand(ctx context.Context, socket string, resource domain.WorkspaceResource, window domain.WorkspaceWindow) error {
	if window.Command == "" {
		return nil
	}
	return a.run(ctx, socket, "send-keys", "-t", resource.Name+":"+window.Name, window.Command, "Enter")
}

func (a TmuxAdapter) configureCellWorkspace(ctx context.Context, socket string, resource domain.WorkspaceResource) error {
	if err := a.run(ctx, socket, "set-environment", "-t", resource.Name, "PARACELL_CELL", resource.CellName); err != nil {
		return err
	}
	if err := a.run(ctx, socket, "set-environment", "-t", resource.Name, "PARACELL_ROOT", a.Root); err != nil {
		return err
	}
	windowTargets := make([]string, 0, len(resource.Windows))
	for _, window := range resource.Windows {
		windowTargets = append(windowTargets, resource.Name+":"+window.Name)
	}
	if len(windowTargets) == 0 {
		windowTargets = append(windowTargets, resource.Name)
	}
	return a.configureWorkspace(ctx, socket, resource.Name, resource.Project, resource.DisplayLabel, windowTargets)
}

func (a TmuxAdapter) UpdateStatusLabel(ctx context.Context, resource domain.WorkspaceResource) error {
	socket := a.locateCellSocket(ctx, resource)
	err := a.run(ctx, socket, "set-option", "-t", resource.Name, "@paracell-status-label", resource.DisplayLabel)
	if err == nil {
		_, err = a.normalizeWindowNames(ctx, socket, resource.Name, resource.DisplayLabel)
		return err
	}
	if strings.Contains(strings.ToLower(err.Error()), "can't find session") {
		return fmt.Errorf("%w: %v", domain.ErrNotFound, err)
	}
	return err
}

func (a TmuxAdapter) configureWorkspace(ctx context.Context, socket string, target string, project string, label string, windowTargets []string) error {
	keyTable := "paracell-" + target
	if err := a.run(ctx, socket, "set-option", "-t", target, "@paracell-project", project); err != nil {
		return err
	}
	if err := a.run(ctx, socket, "set-option", "-t", target, "@paracell-status-label", label); err != nil {
		return err
	}
	if err := a.run(ctx, socket, "set-option", "-t", target, "set-titles", "on"); err != nil {
		return err
	}
	if err := a.run(ctx, socket, "set-option", "-t", target, "set-titles-string", "#{@paracell-project}"); err != nil {
		return err
	}
	if err := a.run(ctx, socket, "set-option", "-t", target, "status-left", "#{@paracell-status-label} "); err != nil {
		return err
	}
	if err := a.run(ctx, socket, "set-option", "-t", target, "status-left-length", "100"); err != nil {
		return err
	}
	statusRight, err := a.output(ctx, socket, "show-option", "-v", "-t", target, "status-right")
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
		if err := a.run(ctx, socket, "set-option", "-t", target, "status-right", statusRight); err != nil {
			return err
		}
	}
	windowFormat := "#W#{?window_flags,#{window_flags}, }"
	if listed, err := a.normalizeWindowNames(ctx, socket, target, label); err != nil {
		return err
	} else if len(listed) > 0 {
		windowTargets = listed
	}
	for _, windowTarget := range windowTargets {
		if err := a.run(ctx, socket, "set-window-option", "-t", windowTarget, "window-status-format", windowFormat); err != nil {
			return err
		}
		if err := a.run(ctx, socket, "set-window-option", "-t", windowTarget, "window-status-current-format", windowFormat); err != nil {
			return err
		}
	}
	newWindowHook := "set-window-option window-status-format '" + windowFormat + "'; set-window-option window-status-current-format '" + windowFormat + "'"
	if err := a.run(ctx, socket, "set-hook", "-t", target, "after-new-window[100]", newWindowHook); err != nil {
		return err
	}
	if err := a.run(ctx, socket, "set-option", "-t", target, "key-table", keyTable); err != nil {
		return err
	}
	if err := a.run(ctx, socket, "set-option", "-t", target, "mouse", "on"); err != nil {
		return err
	}
	if err := a.run(ctx, socket, "set-option", "-t", target, "set-clipboard", "on"); err != nil {
		return err
	}
	if err := a.run(ctx, socket, "bind-key", "-T", keyTable, "MouseDown1Pane", "select-pane", "-t", "=", "\\;", "send-keys", "-M"); err != nil {
		return err
	}
	if err := a.run(ctx, socket, "bind-key", "-T", keyTable, "MouseDrag1Pane", "if-shell", "-F", "#{||:#{pane_in_mode},#{mouse_any_flag}}", "send-keys -M", "copy-mode -M"); err != nil {
		return err
	}
	if err := a.run(ctx, socket, "bind-key", "-T", keyTable, "WheelUpPane", "if-shell", "-F", "#{||:#{alternate_on},#{pane_in_mode},#{mouse_any_flag}}", "send-keys -M", "copy-mode -e"); err != nil {
		return err
	}
	if err := a.run(ctx, socket, "bind-key", "-T", keyTable, "C-t", "next-window"); err != nil {
		return err
	}
	args := []string{"bind-key", "-T", keyTable, "C-p", "display-popup", "-t", target, "-w", "65", "-h", "24"}
	if a.Root != "" {
		args = append(args, "-d", a.Root)
	}
	args = append(args, "-E", "paracell", "view")
	return a.run(ctx, socket, args...)
}

func (a TmuxAdapter) normalizeWindowNames(ctx context.Context, socket string, target string, label string) ([]string, error) {
	listed, err := a.output(ctx, socket, "list-windows", "-t", target, "-F", "#{window_id}\t#{window_name}")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(listed) == "" {
		return nil, nil
	}
	windowTargets := make([]string, 0)
	for _, line := range strings.Split(strings.TrimSpace(listed), "\n") {
		window := strings.SplitN(line, "\t", 2)
		windowTargets = append(windowTargets, window[0])
		if len(window) == 2 {
			name := windowNameWithoutWorkspaceLabel(window[1], label)
			if name != window[1] {
				if err := a.run(ctx, socket, "rename-window", "-t", window[0], name); err != nil {
					return nil, err
				}
			}
		}
	}
	return windowTargets, nil
}

func (a TmuxAdapter) CleanWorkspace(ctx context.Context, resource domain.WorkspaceResource) error {
	return a.killSession(ctx, a.locateCellSocket(ctx, resource), resource)
}

func (a TmuxAdapter) killSession(ctx context.Context, socket string, resource domain.WorkspaceResource) error {
	err := a.run(ctx, socket, "kill-session", "-t", resource.Name)
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "can't find session") {
		return fmt.Errorf("%w: %v", domain.ErrNotFound, err)
	}
	return err
}

func (a TmuxAdapter) EnterWorkspace(ctx context.Context, resource domain.WorkspaceResource) error {
	args, err := a.EnterArgs(ctx, resource)
	if err != nil {
		return err
	}
	return a.Runner.Run(ctx, "tmux", args...)
}

// EnterArgs prepares the cell session and returns the tmux arguments that move
// the current terminal to it.
func (a TmuxAdapter) EnterArgs(ctx context.Context, resource domain.WorkspaceResource) ([]string, error) {
	socket := a.locateCellSocket(ctx, resource)
	if err := a.configureCellWorkspace(ctx, socket, resource); err != nil {
		return nil, err
	}
	return switchTo(socket, resource.Name), nil
}

// EnterCommand is EnterArgs as a command for callers that run it themselves (the TUI).
func (a TmuxAdapter) EnterCommand(ctx context.Context, resource domain.WorkspaceResource) (*exec.Cmd, error) {
	args, err := a.EnterArgs(ctx, resource)
	if err != nil {
		return nil, err
	}
	return exec.CommandContext(ctx, "tmux", args...), nil
}

func (a TmuxAdapter) PrepareWorkspace(ctx context.Context, resource domain.WorkspaceResource) error {
	return a.configureCellWorkspace(ctx, a.locateCellSocket(ctx, resource), resource)
}

func (a TmuxAdapter) EnterRootWorkspace(ctx context.Context, projectName string) error {
	name := rootWorkspaceName(projectName)
	if err := a.ensureRootWorkspace(ctx, name); err != nil {
		return err
	}
	return a.Runner.Run(ctx, "tmux", switchTo("", name)...)
}

func (a TmuxAdapter) ExitWorkspace(ctx context.Context) error {
	if os.Getenv("TMUX") == "" {
		return errors.New("paracell exit must be run inside tmux")
	}
	// Without -L, tmux talks to the server named by $TMUX, i.e. the current one.
	return a.Runner.Run(ctx, "tmux", "detach-client")
}

func (a TmuxAdapter) ensureRootWorkspace(ctx context.Context, name string) error {
	err := a.run(ctx, "", "has-session", "-t", name)
	if err == nil {
		return a.configureRootWorkspace(ctx, name)
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
	if err := a.run(ctx, "", args...); err != nil {
		return err
	}
	return a.configureRootWorkspace(ctx, name)
}

func (a TmuxAdapter) configureRootWorkspace(ctx context.Context, name string) error {
	if err := a.run(ctx, "", "set-environment", "-u", "-t", name, "PARACELL_CELL"); err != nil {
		return err
	}
	if err := a.run(ctx, "", "set-environment", "-t", name, "PARACELL_ROOT", a.Root); err != nil {
		return err
	}
	return a.configureWorkspace(ctx, "", name, strings.TrimSuffix(name, "-root"), "root", []string{name})
}

func rootWorkspaceName(project string) string {
	return project + "-root"
}
