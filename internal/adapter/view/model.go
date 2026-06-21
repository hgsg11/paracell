package view

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hgsg11/paracell/internal/domain"
)

type Action string
type FocusArea string

const (
	ActionNone   Action = ""
	ActionQuit   Action = "quit"
	ActionExit   Action = "exit"
	ActionEnter  Action = "enter"
	ActionDelete Action = "delete"

	FocusCells     FocusArea = "cells"
	FocusExit      FocusArea = "exit"
	FocusTemplates FocusArea = "templates"
)

type Result struct {
	Action Action
	Cell   domain.Cell
}

type forkResultMsg struct {
	cell domain.Cell
	err  error
}

type Model struct {
	Cells            []domain.Cell
	Templates        []string
	Focus            FocusArea
	Selected         int
	TemplateSelected int
	Quitting         bool
	AwaitingDelete   bool
	AwaitingFork     bool
	IssueInputActive bool
	ForkTemplate     string
	IssueInput       string
	Error            string
	Width            int
	Result           Result
	Enter            func(domain.Cell) tea.Cmd
	Fork             func(issue string, template string) tea.Cmd
	Delete           func(domain.Cell) error
	MarkDone         func(domain.Cell) (domain.Cell, error)
	Reload           func() ([]domain.Cell, error)
}

func NewModel(cells []domain.Cell, templates ...[]string) Model {
	model := Model{
		Cells: append([]domain.Cell(nil), cells...),
		Focus: FocusCells,
	}
	if len(templates) > 0 {
		model.Templates = append([]string(nil), templates[0]...)
	}
	return model
}

func (m Model) Init() tea.Cmd {
	return m.refreshCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		if m.IssueInputActive {
			switch msg.Type {
			case tea.KeyRunes:
				m.IssueInput += string(msg.Runes)
				return m, nil
			case tea.KeyBackspace:
				if len(m.IssueInput) > 0 {
					m.IssueInput = m.IssueInput[:len(m.IssueInput)-1]
				}
				return m, nil
			case tea.KeyEnter:
				if strings.TrimSpace(m.IssueInput) == "" {
					m.Error = "issue is required"
					return m, nil
				}
				if m.Fork == nil {
					return m, nil
				}
				return m, m.Fork(m.IssueInput, m.ForkTemplate)
			case tea.KeyEsc:
				m.IssueInputActive = false
				m.AwaitingFork = false
				m.ForkTemplate = ""
				m.IssueInput = ""
				return m, nil
			}
		}
		if key == "tab" {
			if m.Focus == FocusCells {
				m.Focus = FocusExit
			} else if m.Focus == FocusExit {
				m.Focus = FocusTemplates
			} else {
				m.Focus = FocusCells
			}
			return m, nil
		}
		if key == "q" {
			m.Quitting = true
			m.Result = Result{Action: ActionQuit}
			return m, tea.Quit
		}
		if m.AwaitingDelete {
			m.AwaitingDelete = false
			if key == "d" {
				if m.isExitSelected() {
					m.Error = "exit paracell cannot be cleaned"
					return m, nil
				}
				if len(m.Cells) == 0 {
					m.Error = "no cells available"
					return m, nil
				}
				cell := m.Cells[m.Selected]
				del := m.Delete
				return m, func() tea.Msg {
					if del == nil {
						return deleteResultMsg{cell: cell, err: errors.New("delete handler is not configured")}
					}
					return deleteResultMsg{cell: cell, err: del(cell)}
				}
			}
		}
		switch key {
		case "j":
			if m.Focus == FocusTemplates {
				if m.TemplateSelected < m.lastTemplateIndex() {
					m.TemplateSelected++
				}
			} else if m.Focus == FocusCells && m.Selected < m.lastSelectableIndex() {
				m.Selected++
			}
		case "k":
			if m.Focus == FocusTemplates {
				if m.TemplateSelected > 0 {
					m.TemplateSelected--
				}
			} else if m.Focus == FocusCells && m.Selected > 0 {
				m.Selected--
			}
		case "d":
			if m.Focus != FocusCells {
				return m, nil
			}
			m.AwaitingDelete = true
			return m, nil
		case "y":
			if m.Focus == FocusTemplates {
				if m.AwaitingFork {
					m.IssueInputActive = true
					if len(m.Templates) > 0 && m.TemplateSelected < len(m.Templates) {
						m.ForkTemplate = m.Templates[m.TemplateSelected]
					}
					return m, nil
				}
				m.AwaitingFork = true
				return m, nil
			}
		case "enter":
			if m.Focus == FocusExit {
				m.Error = "exit paracell cannot be marked done"
				return m, nil
			}
			if m.Focus != FocusCells {
				return m, nil
			}
			if len(m.Cells) == 0 {
				m.Error = "no cells available"
				return m, nil
			}
			cell := m.Cells[m.Selected]
			markDone := m.MarkDone
			return m, func() tea.Msg {
				if markDone == nil {
					return markDoneResultMsg{cell: cell, err: errors.New("mark done handler is not configured")}
				}
				updated, err := markDone(cell)
				return markDoneResultMsg{cell: updated, err: err}
			}
		case "l":
			if m.Focus == FocusExit {
				m.Result = Result{Action: ActionExit}
				m.Quitting = true
				return m, tea.Quit
			}
			if m.Focus != FocusCells {
				return m, nil
			}
			cell := m.Cells[m.Selected]
			enter := m.Enter
			if enter == nil {
				return m, EnterFailureCmd(cell, errors.New("enter handler is not configured"))
			}
			return m, enter(cell)
		}
	case tea.WindowSizeMsg:
		m.Width = msg.Width
	case enterResultMsg:
		if msg.err != nil {
			m.Error = msg.err.Error()
			return m, nil
		}
		m.Error = ""
		m.Result = Result{Action: ActionEnter, Cell: msg.cell}
		m.Quitting = true
		return m, tea.Quit
	case deleteResultMsg:
		if msg.err != nil {
			m.Error = msg.err.Error()
			return m, nil
		}
		m.Error = ""
		if len(m.Cells) == 0 {
			return m, nil
		}
		index := -1
		for i, cell := range m.Cells {
			if cell.ID == msg.cell.ID {
				index = i
				break
			}
		}
		if index < 0 {
			return m, nil
		}
		m.Cells = append(append([]domain.Cell{}, m.Cells[:index]...), m.Cells[index+1:]...)
		if m.Selected >= len(m.Cells) && m.Selected > 0 {
			m.Selected--
		}
		m.Result = Result{Action: ActionDelete, Cell: msg.cell}
		return m, nil
	case markDoneResultMsg:
		if msg.err != nil {
			m.Error = msg.err.Error()
			return m, nil
		}
		m.Error = ""
		if len(m.Cells) == 0 {
			return m, nil
		}
		index := -1
		for i, cell := range m.Cells {
			if cell.ID == msg.cell.ID {
				index = i
				break
			}
		}
		if index < 0 {
			return m, nil
		}
		m.Cells[index] = msg.cell
		return m, nil
	case forkResultMsg:
		if msg.err != nil {
			m.Error = msg.err.Error()
			return m, nil
		}
		m.Error = ""
		if m.Reload != nil {
			cells, err := m.Reload()
			if err != nil {
				m.Error = err.Error()
				return m, nil
			}
			m.Cells = cells
		}
		m.IssueInputActive = false
		m.AwaitingFork = false
		m.ForkTemplate = ""
		m.IssueInput = ""
		return m, nil
	case refreshMsg:
		if m.Reload == nil {
			return m, m.refreshCmd()
		}
		cells, err := m.Reload()
		if err != nil {
			m.Error = err.Error()
			return m, nil
		}
		selectedID := ""
		if m.Selected >= 0 && m.Selected < len(m.Cells) {
			selectedID = m.Cells[m.Selected].ID
		}
		m.Cells = cells
		if selectedID != "" {
			for i, cell := range m.Cells {
				if cell.ID == selectedID {
					m.Selected = i
					break
				}
			}
		}
		if m.Selected >= len(m.Cells) {
			m.Selected = len(m.Cells)
		}
		return m, m.refreshCmd()
	}
	return m, nil
}

func (m Model) lastTemplateIndex() int {
	if len(m.Templates) == 0 {
		return 0
	}
	return len(m.Templates) - 1
}

func (m Model) isExitSelected() bool {
	return m.Focus == FocusExit
}

func (m Model) lastSelectableIndex() int {
	if len(m.Cells) == 0 {
		return 0
	}
	return len(m.Cells) - 1
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString("  TEMPLATES\n")
	if len(m.Templates) == 0 {
		prefix := " "
		if m.Focus == FocusTemplates {
			prefix = ">"
		}
		fmt.Fprintf(&b, "%s no templates\n", prefix)
	} else {
		for i, template := range m.Templates {
			prefix := " "
			if m.Focus == FocusTemplates && i == m.TemplateSelected {
				prefix = ">"
			}
			fmt.Fprintf(&b, "%s %s\n", prefix, template)
		}
	}
	b.WriteString("\n")
	nameWidth, templateWidth, statusWidth := tableWidths(m.Cells)
	fmt.Fprintf(&b, "  %s  %s  %s  DONE\n", padded("NAME", nameWidth), padded("TEMPLATE", templateWidth), padded("STATUS", statusWidth))
	if len(m.Cells) == 0 {
		prefix := " "
		if m.Focus == FocusCells {
			prefix = ">"
		}
		fmt.Fprintf(&b, "%s no cells\n", prefix)
	}
	for i, cell := range m.Cells {
		prefix := " "
		if m.Focus == FocusCells && i == m.Selected {
			prefix = ">"
		}
		done := "[ ]"
		if cell.IsDone() {
			done = "[x]"
		}
		fmt.Fprintf(&b, "%s %s  %s  %s  %s\n", prefix, padded(cell.Name, nameWidth), padded(cell.Template, templateWidth), padded(string(cell.Status()), statusWidth), done)
	}
	prefix := " "
	if m.isExitSelected() {
		prefix = ">"
	}
	fmt.Fprintf(&b, "\n%s exit paracell\n", prefix)
	b.WriteString(statusLine(m))
	return b.String()
}

func tableWidths(cells []domain.Cell) (int, int, int) {
	nameWidth := lipgloss.Width("NAME")
	templateWidth := lipgloss.Width("TEMPLATE")
	statusWidth := lipgloss.Width("STATUS")
	for _, cell := range cells {
		nameWidth = max(nameWidth, lipgloss.Width(cell.Name))
		templateWidth = max(templateWidth, lipgloss.Width(cell.Template))
		statusWidth = max(statusWidth, lipgloss.Width(string(cell.Status())))
	}
	return nameWidth, templateWidth, statusWidth
}

func padded(value string, width int) string {
	padding := width - lipgloss.Width(value)
	if padding <= 0 {
		return value
	}
	return value + strings.Repeat(" ", padding)
}

func errorLine(message string, width int) string {
	if message == "" {
		return "\n"
	}
	line := "error: " + singleLine(message)
	if width <= 0 {
		width = 80
	}
	return clipWidth(line, width) + "\n"
}

func statusLine(m Model) string {
	if m.IssueInputActive {
		return clipWidth("issue: "+m.IssueInput, widthOrDefault(m.Width)) + "\n"
	}
	return errorLine(m.Error, m.Width)
}

func widthOrDefault(width int) int {
	if width <= 0 {
		return 80
	}
	return width
}

func singleLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func clipWidth(value string, width int) string {
	if lipgloss.Width(value) <= width {
		return value
	}
	var b strings.Builder
	for _, r := range value {
		if lipgloss.Width(b.String()+string(r)) > width {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (m Model) refreshCmd() tea.Cmd {
	if m.Reload == nil {
		return nil
	}
	return tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg {
		return refreshMsg{}
	})
}

type enterResultMsg struct {
	cell domain.Cell
	err  error
}

func EnterProcessCmd(cell domain.Cell, cmd *exec.Cmd) tea.Cmd {
	return tea.Exec(newCapturedExecCommand(cmd), func(err error) tea.Msg {
		return enterResultMsg{cell: cell, err: err}
	})
}

type capturedExecCommand struct {
	cmd    *exec.Cmd
	stderr bytes.Buffer
}

func newCapturedExecCommand(cmd *exec.Cmd) *capturedExecCommand {
	wrapped := &capturedExecCommand{cmd: cmd}
	if wrapped.cmd.Stderr == nil {
		wrapped.cmd.Stderr = &wrapped.stderr
	}
	return wrapped
}

func (c *capturedExecCommand) SetStdin(r io.Reader) {
	if c.cmd.Stdin == nil {
		c.cmd.Stdin = r
	}
}

func (c *capturedExecCommand) SetStdout(w io.Writer) {
	if c.cmd.Stdout == nil {
		c.cmd.Stdout = w
	}
}

func (c *capturedExecCommand) SetStderr(io.Writer) {
	if c.cmd.Stderr == nil {
		c.cmd.Stderr = &c.stderr
	}
}

func (c *capturedExecCommand) Run() error {
	err := c.cmd.Run()
	if err == nil {
		return nil
	}
	output := strings.TrimSpace(c.stderr.String())
	if output == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, output)
}

func EnterFailureCmd(cell domain.Cell, err error) tea.Cmd {
	return func() tea.Msg {
		return enterResultMsg{cell: cell, err: err}
	}
}

func ForkResultCmd(cell domain.Cell, err error) tea.Cmd {
	return func() tea.Msg {
		return forkResultMsg{cell: cell, err: err}
	}
}

type deleteResultMsg struct {
	cell domain.Cell
	err  error
}

type markDoneResultMsg struct {
	cell domain.Cell
	err  error
}

type refreshMsg struct{}
