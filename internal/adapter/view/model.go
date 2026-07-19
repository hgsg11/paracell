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
	ActionGoRoot Action = "go-root"
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
	CurrentCell      string
	Focus            FocusArea
	Selected         int
	TemplateSelected int
	StatusFrame      int
	Quitting         bool
	AwaitingDelete   bool
	AwaitingFork     bool
	IssueInputActive bool
	ForkTemplate     string
	IssueInput       string
	Error            string
	Width            int
	Height           int
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

var pendingStatusFrames = []string{"..", "o.", ".o"}

const maxTemplateDisplayWidth = 16
const maxIssueDisplayWidth = 20

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
				m.IssueInput = removeLastRune(m.IssueInput)
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
				return resetForkInput(m), nil
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
					m.Error = "go root cannot be cleaned"
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
		case "h":
			switch m.Focus {
			case FocusCells:
				m.Focus = FocusTemplates
			case FocusTemplates:
				m.Focus = FocusExit
			case FocusExit:
				m.Focus = FocusTemplates
			}
		case "l":
			switch m.Focus {
			case FocusTemplates:
				m.Focus = FocusCells
			case FocusCells:
				m.Focus = FocusExit
			case FocusExit:
				m.Focus = FocusCells
			}
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
				m.Error = "go root cannot be marked done"
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
		case " ":
			if m.Focus == FocusExit {
				m.Result = Result{Action: ActionGoRoot}
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
		m.Height = msg.Height
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
		m.StatusFrame = (m.StatusFrame + 1) % len(pendingStatusFrames)
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
	b.WriteString(renderHeaderLine(m))
	b.WriteString(renderSplitLayout(m))
	b.WriteString(renderExitLine(m))
	b.WriteString(statusLine(m))
	return b.String()
}

func renderHeaderLine(m Model) string {
	focus := "cells"
	switch m.Focus {
	case FocusTemplates:
		focus = "templates"
	case FocusExit:
		focus = "go root"
	}
	return fmt.Sprintf("paracell / %s\n\n", focus)
}

func renderSplitLayout(m Model) string {
	leftWidth, rightWidth := paneWidths(m.Width)
	paneHeight := paneHeight(m)
	left := renderTemplatesPane(m, leftWidth, paneHeight)
	right := renderCellsPane(m, rightWidth, paneHeight)
	var b strings.Builder
	b.WriteString(joinSideBySide(left, right, " │ "))
	return b.String()
}

func renderTemplatesPane(m Model, width int, height int) []string {
	lines := make([]string, 0, len(m.Templates)+1)
	selected := m.TemplateSelected
	if len(m.Templates) == 0 {
		lines = append(lines, "no templates")
		lines = append(lines, renderIssueInputLine(m))
	} else {
		lines = append(lines, m.Templates...)
		lines = append(lines, renderIssueInputLine(m))
	}
	if m.IssueInputActive {
		selected = len(lines) - 1
	}
	lines, selected = visibleRows(lines, selected, height)
	for i := range lines {
		lines[i] = renderPaneLine(lines[i], width, m.Focus == FocusTemplates && i == selected)
	}
	return lines
}

func renderCellsPane(m Model, width int, height int) []string {
	lines := make([]string, 0, max(1, len(m.Cells)))
	if len(m.Cells) == 0 {
		lines = append(lines, "no cells")
	} else {
		nameWidth, templateWidth := cellWidths(m.Cells)
		for _, cell := range m.Cells {
			done := "[ ]"
			if cell.IsDone() {
				done = "[x]"
			}
			lines = append(lines, fmt.Sprintf("%s %s  %s  %s  %s", currentCellMarker(cell, m.CurrentCell), padded(ellipsize(cell.Name, maxIssueDisplayWidth), nameWidth), padded(ellipsize(cell.Template, maxTemplateDisplayWidth), templateWidth), done, renderCellStatus(cell, m.StatusFrame)))
		}
	}
	selected := m.Selected
	lines, selected = visibleRows(lines, selected, height)
	for i := range lines {
		lines[i] = renderPaneLine(lines[i], width, m.Focus == FocusCells && i == selected)
	}
	return lines
}

func renderExitLine(m Model) string {
	line := "go root"
	line = padded(clipWidth(line, widthOrDefault(m.Width)), widthOrDefault(m.Width))
	if m.isExitSelected() {
		line = renderSelectedLine(line)
	}
	return line + "\n"
}

func paneWidths(width int) (int, int) {
	contentWidth := max(2, widthOrDefault(width)-lipgloss.Width(" │ "))
	leftWidth := contentWidth * 3 / 10
	return leftWidth, contentWidth - leftWidth
}

func paneHeight(m Model) int {
	templateRows, cellRows := naturalPaneHeights(m)
	contentHeight := max(templateRows, cellRows)
	if m.Height > 0 {
		// header, its spacer, go root, and status each occupy one terminal row.
		return min(contentHeight, max(1, m.Height-4))
	}
	return contentHeight
}

func naturalPaneHeights(m Model) (int, int) {
	return max(1, len(m.Templates)) + 1, max(1, len(m.Cells))
}

func visibleRows(lines []string, selected int, height int) ([]string, int) {
	height = max(1, height)
	selected = min(max(0, selected), len(lines)-1)
	start := 0
	if selected >= height {
		start = selected - height + 1
	}
	end := min(len(lines), start+height)
	visible := append([]string(nil), lines[start:end]...)
	for len(visible) < height {
		visible = append(visible, "")
	}
	return visible, selected - start
}

func renderPaneLine(value string, width int, selected bool) string {
	line := padded(clipWidth(value, width), width)
	if selected {
		return renderSelectedLine(line)
	}
	return line
}

func joinSideBySide(left []string, right []string, separator string) string {
	leftWidth := maxLineWidth(left)
	rightWidth := maxLineWidth(right)
	lineCount := max(len(left), len(right))
	var b strings.Builder
	for i := 0; i < lineCount; i++ {
		leftLine := ""
		if i < len(left) {
			leftLine = left[i]
		}
		rightLine := ""
		if i < len(right) {
			rightLine = right[i]
		}
		b.WriteString(padded(leftLine, leftWidth))
		b.WriteString(separator)
		b.WriteString(padded(rightLine, rightWidth))
		b.WriteByte('\n')
	}
	return b.String()
}

func maxLineWidth(lines []string) int {
	width := 0
	for _, line := range lines {
		width = max(width, lipgloss.Width(line))
	}
	return width
}

func cellWidths(cells []domain.Cell) (int, int) {
	nameWidth := lipgloss.Width("NAME")
	templateWidth := lipgloss.Width("TEMPLATE")
	for _, cell := range cells {
		nameWidth = max(nameWidth, lipgloss.Width(ellipsize(cell.Name, maxIssueDisplayWidth)))
		templateWidth = max(templateWidth, lipgloss.Width(ellipsize(cell.Template, maxTemplateDisplayWidth)))
	}
	return nameWidth, templateWidth
}

func renderCellStatus(cell domain.Cell, frame int) string {
	switch cell.Status() {
	case domain.Pending:
		return pendingStatusFrames[frame%len(pendingStatusFrames)]
	case domain.Ready:
		return ""
	default:
		return "  "
	}
}

func renderSelectedLine(value string) string {
	return "\x1b[7m" + value + "\x1b[0m"
}

func renderIssueInputLine(m Model) string {
	if m.IssueInputActive {
		return "issue: " + m.IssueInput
	}
	return ""
}

func removeLastRune(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return value
	}
	return string(runes[:len(runes)-1])
}

func resetForkInput(m Model) Model {
	m.IssueInputActive = false
	m.AwaitingFork = false
	m.ForkTemplate = ""
	m.IssueInput = ""
	return m
}

func currentCellMarker(cell domain.Cell, currentCell string) string {
	if currentCell != "" && cell.Name == currentCell {
		return "*"
	}
	return " "
}

func ellipsize(value string, width int) string {
	if width <= 0 || lipgloss.Width(value) <= width {
		return value
	}
	if width <= 3 {
		return strings.Repeat(".", width)
	}
	var b strings.Builder
	for _, r := range value {
		if lipgloss.Width(b.String()+string(r)+"...") > width {
			break
		}
		b.WriteRune(r)
	}
	return b.String() + "..."
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
