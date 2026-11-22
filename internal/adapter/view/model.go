package view

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hgsg11/paracell/internal/adapter/logging"
	"github.com/hgsg11/paracell/internal/query"
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

const (
	cellStatusPending = "pending"
	cellStatusReady   = "ready"

	cellProgressInProgress = "in_progress"
	cellProgressInReview   = "in_review"
	cellProgressMerged     = "merged"
	cellProgressAbandoned  = "abandoned"
)

// Result reports the action that ended the view and the name of the Cell it acted on.
type Result struct {
	Action Action
	Cell   string
}

type forkResultMsg struct {
	err error
}

type Model struct {
	Cells []query.CellView
	// Waiting is the queue entries that wait for earlier Cells. They are shown
	// below the Cells and cannot be selected.
	Waiting          []query.QueueView
	Templates        []string
	CurrentCell      string
	Focus            FocusArea
	Selected         int
	TemplateSelected int
	StatusFrame      int
	Quitting         bool
	AwaitingDelete   bool
	// AwaitingProgress is the progress, merged or abandoned, waiting for the
	// confirming second key press on the Cell named AwaitingProgressCell.
	AwaitingProgress     string
	AwaitingProgressCell string
	AwaitingFork         bool
	ForksInProgress      int
	IssueInputActive     bool
	ForkTemplate         string
	IssueInput           string
	Error                string
	Logs                 []logging.Entry
	Width                int
	Height               int
	Result               Result
	// Enter, Delete, MarkMerged, and MarkAbandoned receive the Cell name.
	Enter         func(cellName string) tea.Cmd
	Fork          func(issue string, template string) tea.Cmd
	Delete        func(cellName string) error
	MarkMerged    func(cellName string) error
	MarkAbandoned func(cellName string) error
	Reload        func() ([]query.CellView, error)
	ReloadQueue   func() ([]query.QueueView, error)
	Logger        *logging.Logger
}

func NewModel(cells []query.CellView, templates ...[]string) Model {
	model := Model{
		Cells: append([]query.CellView(nil), cells...),
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
const maxTemplatePaneWidth = 15
const maxCellPaneWidth = 47
const maxLayoutWidth = maxTemplatePaneWidth + 3 + maxCellPaneWidth

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.refreshCmd(), m.waitLogCmd())
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
					m.setError("issue is required")
					return m, nil
				}
				if m.Fork == nil {
					return m, nil
				}
				issue := m.IssueInput
				template := m.ForkTemplate
				forkCmd := m.Fork(issue, template)
				m = resetForkInput(m)
				if forkCmd == nil {
					m.setError("cell creation did not start")
					return m, nil
				}
				m.ForksInProgress++
				return m, forkCmd
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
					m.setError("go root cannot be cleaned")
					return m, nil
				}
				if len(m.Cells) == 0 {
					m.setError("no cells available")
					return m, nil
				}
				name := m.Cells[m.Selected].Name
				del := m.Delete
				return m, func() tea.Msg {
					if del == nil {
						return deleteResultMsg{name: name, err: errors.New("delete handler is not configured")}
					}
					return deleteResultMsg{name: name, err: del(name)}
				}
			}
		}
		if m.AwaitingProgress != "" {
			progress, name := m.AwaitingProgress, m.AwaitingProgressCell
			m.AwaitingProgress, m.AwaitingProgressCell = "", ""
			if key == progressKey(progress) {
				return m, m.markProgressCmd(name, progress)
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
		case "m", "a":
			// Merged and abandoned cannot be undone, so the same key must be
			// pressed twice.
			progress := cellProgressMerged
			if key == "a" {
				progress = cellProgressAbandoned
			}
			if m.Focus == FocusExit {
				m.setError("go root cannot be marked " + progress)
				return m, nil
			}
			if m.Focus != FocusCells {
				return m, nil
			}
			if len(m.Cells) == 0 {
				m.setError("no cells available")
				return m, nil
			}
			cell := m.Cells[m.Selected]
			if isTerminalProgress(cell.Progress) {
				m.setError(fmt.Sprintf("cell %s is already %s and cannot be changed", cell.Name, cell.Progress))
				return m, nil
			}
			m.AwaitingProgress, m.AwaitingProgressCell = progress, cell.Name
			return m, nil
		case " ":
			if m.Focus == FocusExit {
				m.Result = Result{Action: ActionGoRoot}
				m.Quitting = true
				return m, tea.Quit
			}
			if m.Focus != FocusCells {
				return m, nil
			}
			name := m.Cells[m.Selected].Name
			enter := m.Enter
			if enter == nil {
				return m, EnterFailureCmd(name, errors.New("enter handler is not configured"))
			}
			return m, enter(name)
		}
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
	case enterResultMsg:
		if msg.err != nil {
			m.setError(msg.err.Error())
			return m, nil
		}
		m.Error = ""
		m.Result = Result{Action: ActionEnter, Cell: msg.name}
		m.Quitting = true
		return m, tea.Quit
	case deleteResultMsg:
		if msg.err != nil {
			m.setError(msg.err.Error())
			return m, nil
		}
		m.Error = ""
		index := m.cellIndex(msg.name)
		if index < 0 {
			return m, nil
		}
		m.Cells = append(append([]query.CellView{}, m.Cells[:index]...), m.Cells[index+1:]...)
		if m.Selected >= len(m.Cells) && m.Selected > 0 {
			m.Selected--
		}
		m.Result = Result{Action: ActionDelete, Cell: msg.name}
		return m, nil
	case markProgressResultMsg:
		if msg.err != nil {
			m.setError(msg.err.Error())
			return m, nil
		}
		m.Error = ""
		index := m.cellIndex(msg.name)
		if index < 0 {
			return m, nil
		}
		// Show the change at once; the periodic reload corrects any other drift.
		m.Cells = append([]query.CellView(nil), m.Cells...)
		m.Cells[index].Progress = msg.progress
		return m, nil
	case forkResultMsg:
		if m.ForksInProgress > 0 {
			m.ForksInProgress--
		}
		var reloadErr error
		if m.Reload != nil {
			cells, err := m.Reload()
			if err != nil {
				reloadErr = err
			} else {
				m.Cells = cells
			}
		}
		reloadErr = errors.Join(reloadErr, m.reloadWaiting())
		if msg.err != nil || reloadErr != nil {
			m.setError(errors.Join(msg.err, reloadErr).Error())
			return m, nil
		}
		m.Error = ""
		return m, nil
	case refreshMsg:
		m.StatusFrame = (m.StatusFrame + 1) % len(pendingStatusFrames)
		if m.Reload == nil {
			return m, m.refreshCmd()
		}
		cells, err := m.Reload()
		if err != nil {
			m.setError(err.Error())
			return m, nil
		}
		if err := m.reloadWaiting(); err != nil {
			m.setError(err.Error())
			return m, nil
		}
		selectedName := ""
		hasSelectedCell := false
		if m.Selected >= 0 && m.Selected < len(m.Cells) {
			selectedName = m.Cells[m.Selected].Name
			hasSelectedCell = true
		}
		m.Cells = cells
		if hasSelectedCell {
			if index := m.cellIndex(selectedName); index >= 0 {
				m.Selected = index
			}
		}
		if m.Selected >= len(m.Cells) {
			m.Selected = len(m.Cells)
		}
		return m, m.refreshCmd()
	case logEntryMsg:
		m.Logs = append(m.Logs, logging.Entry(msg))
		const maxInMemoryLogEntries = 2000
		if len(m.Logs) > maxInMemoryLogEntries {
			m.Logs = append([]logging.Entry(nil), m.Logs[len(m.Logs)-maxInMemoryLogEntries:]...)
		}
		return m, m.waitLogCmd()
	}
	return m, nil
}

// reloadWaiting replaces the waiting entries with the stored ones.
func (m *Model) reloadWaiting() error {
	if m.ReloadQueue == nil {
		return nil
	}
	entries, err := m.ReloadQueue()
	if err != nil {
		return err
	}
	m.Waiting = waitingEntries(entries)
	return nil
}

func waitingEntries(entries []query.QueueView) []query.QueueView {
	var waiting []query.QueueView
	for _, entry := range entries {
		if entry.State == query.QueueStateWaiting {
			waiting = append(waiting, entry)
		}
	}
	return waiting
}

// markProgressCmd runs the handler that marks the Cell merged or abandoned.
func (m Model) markProgressCmd(name string, progress string) tea.Cmd {
	change, handlerName := m.MarkMerged, "mark merged"
	if progress == cellProgressAbandoned {
		change, handlerName = m.MarkAbandoned, "mark abandoned"
	}
	return func() tea.Msg {
		if change == nil {
			return markProgressResultMsg{name: name, err: errors.New(handlerName + " handler is not configured")}
		}
		return markProgressResultMsg{name: name, progress: progress, err: change(name)}
	}
}

func progressKey(progress string) string {
	if progress == cellProgressAbandoned {
		return "a"
	}
	return "m"
}

func isTerminalProgress(progress string) bool {
	return progress == cellProgressMerged || progress == cellProgressAbandoned
}

// renderCellProgress shortens the progress so that the Cell row stays narrow.
func renderCellProgress(progress string) string {
	label := "-"
	switch progress {
	case cellProgressInProgress:
		label = "impl"
	case cellProgressInReview:
		label = "review"
	case cellProgressMerged:
		label = "merged"
	case cellProgressAbandoned:
		label = "abandon"
	}
	return padded(label, lipgloss.Width("abandon"))
}

// cellIndex returns the position of the Cell with the given name, or -1.
func (m Model) cellIndex(name string) int {
	for i, cell := range m.Cells {
		if cell.Name == name {
			return i
		}
	}
	return -1
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
	b.WriteString(renderIssueInputLine(m))
	b.WriteByte('\n')
	b.WriteString(renderExitLine(m))
	if logsEnabled(m) {
		b.WriteString(renderLogArea(m))
	} else {
		b.WriteString(statusLine(m))
	}
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
	lines := make([]string, 0, max(1, len(m.Templates)))
	selected := m.TemplateSelected
	if len(m.Templates) == 0 {
		lines = append(lines, "no templates")
	} else {
		for _, template := range m.Templates {
			lines = append(lines, ellipsize(template, width))
		}
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
			lines = append(lines, fmt.Sprintf("%s %s  %s  %s  %s%s", currentCellMarker(cell, m.CurrentCell), padded(ellipsize(cell.Label, maxIssueDisplayWidth), nameWidth), padded(ellipsize(cell.Template, maxTemplateDisplayWidth), templateWidth), renderCellProgress(cell.Progress), renderCellStatus(cell, m.StatusFrame), renderCellReview(cell)))
		}
	}
	// Waiting entries follow the Cells, so the selection index stays a Cell index.
	for _, entry := range m.Waiting {
		lines = append(lines, renderWaitingEntry(entry))
	}
	selected := m.Selected
	lines, selected = visibleRows(lines, selected, height)
	for i := range lines {
		lines[i] = renderPaneLine(lines[i], width, m.Focus == FocusCells && i == selected)
	}
	return lines
}

// renderWaitingEntry shows a waiting entry and the issues it waits for.
func renderWaitingEntry(entry query.QueueView) string {
	label := entry.Note
	if label == "" {
		label = entry.Issue
	}
	line := "  " + ellipsize(label, maxIssueDisplayWidth) + "  wait"
	if len(entry.BlockedBy) > 0 {
		line += " " + strings.Join(entry.BlockedBy, ",")
	}
	return line
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
	leftWidth := min(maxTemplatePaneWidth, max(1, contentWidth*3/10))
	rightWidth := min(maxCellPaneWidth, max(1, contentWidth-leftWidth))
	return leftWidth, rightWidth
}

func paneHeight(m Model) int {
	templateRows, cellRows := naturalPaneHeights(m)
	contentHeight := max(templateRows, cellRows)
	if m.Height > 0 {
		// The existing layout reserves its header, input, exit, and status rows.
		reserved := 5
		if logsEnabled(m) {
			// Fill the remaining pane while reserving the log rows at the bottom edge.
			reserved = logAreaHeight(m) + 4
			return max(1, m.Height-reserved)
		}
		return min(contentHeight, max(1, m.Height-reserved))
	}
	return contentHeight
}

func naturalPaneHeights(m Model) (int, int) {
	return max(1, len(m.Templates)), max(1, len(m.Cells)) + len(m.Waiting)
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

func cellWidths(cells []query.CellView) (int, int) {
	nameWidth := lipgloss.Width("NAME")
	templateWidth := lipgloss.Width("TEMPLATE")
	for _, cell := range cells {
		nameWidth = max(nameWidth, lipgloss.Width(ellipsize(cell.Label, maxIssueDisplayWidth)))
		templateWidth = max(templateWidth, lipgloss.Width(ellipsize(cell.Template, maxTemplateDisplayWidth)))
	}
	return nameWidth, templateWidth
}

func renderCellStatus(cell query.CellView, frame int) string {
	switch cell.Status {
	case cellStatusPending:
		return pendingStatusFrames[frame%len(pendingStatusFrames)]
	case cellStatusReady:
		return ""
	default:
		return "  "
	}
}

// renderCellReview shows the pull requests of a Cell in review, marked with
// "!" when a person has to choose merged or abandoned.
func renderCellReview(cell query.CellView) string {
	if len(cell.PullRequests) == 0 {
		return ""
	}
	marker := ""
	if cell.NeedsDecision {
		marker = "!"
	}
	return " " + marker + "#" + strings.Join(cell.PullRequests, ",#")
}

func renderSelectedLine(value string) string {
	return "\x1b[7m" + value + "\x1b[0m"
}

func renderIssueInputLine(m Model) string {
	if m.IssueInputActive {
		return "issue: " + m.IssueInput
	}
	if m.AwaitingProgress != "" {
		return fmt.Sprintf("mark %s %s? press %s again (cannot be undone)", m.AwaitingProgressCell, m.AwaitingProgress, progressKey(m.AwaitingProgress))
	}
	if m.Focus == FocusCells && m.Selected >= 0 && m.Selected < len(m.Cells) && m.Cells[m.Selected].NeedsDecision {
		return fmt.Sprintf("! %s: PRs are partly merged and partly closed; choose merged or abandon", m.Cells[m.Selected].Name)
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

func currentCellMarker(cell query.CellView, currentCell string) string {
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
	return errorLine(m.Error, widthOrDefault(m.Width))
}

func logsEnabled(m Model) bool {
	return m.Logger != nil || len(m.Logs) > 0
}

func logAreaHeight(m Model) int {
	if m.Height <= 0 {
		return 4
	}
	return max(3, m.Height/3)
}

func renderLogArea(m Model) string {
	width := m.Width
	if width <= 0 {
		width = maxLayoutWidth
	}
	lines := make([]string, 0, len(m.Logs))
	for _, entry := range m.Logs {
		for _, line := range strings.Split(entry.String(), "\n") {
			lines = append(lines, wrapLine(line, width)...)
		}
	}
	height := logAreaHeight(m)
	if len(lines) > height {
		lines = lines[len(lines)-height:]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n") + "\n"
}

func wrapLine(value string, width int) []string {
	if value == "" {
		return []string{""}
	}
	lines := make([]string, 0, 1)
	var line strings.Builder
	lineWidth := 0
	for _, r := range value {
		runeWidth := lipgloss.Width(string(r))
		if line.Len() > 0 && lineWidth+runeWidth > width {
			lines = append(lines, line.String())
			line.Reset()
			lineWidth = 0
		}
		line.WriteRune(r)
		lineWidth += runeWidth
	}
	lines = append(lines, line.String())
	return lines
}

func (m *Model) setError(message string) {
	m.Error = message
	if m.Logger == nil {
		return
	}
	if err := m.Logger.Write(logging.LevelError, "paracell", message); err != nil {
		m.Error = message + "; log error: " + err.Error()
		m.Logs = append(m.Logs, logging.Entry{
			Time:    time.Now(),
			Level:   logging.LevelError,
			Source:  "paracell",
			Content: m.Error,
		})
	}
}

func widthOrDefault(width int) int {
	if width <= 0 || width > maxLayoutWidth {
		return maxLayoutWidth
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

func (m Model) waitLogCmd() tea.Cmd {
	if m.Logger == nil {
		return nil
	}
	return func() tea.Msg {
		return logEntryMsg(<-m.Logger.Entries())
	}
}

type enterResultMsg struct {
	name string
	err  error
}

func EnterLoggedProcessCmd(cellName string, cmd *exec.Cmd, logger *logging.Logger) tea.Cmd {
	return tea.Exec(newLoggedCapturedExecCommand(cmd, logger), func(err error) tea.Msg {
		return enterResultMsg{name: cellName, err: err}
	})
}

type capturedExecCommand struct {
	cmd    *exec.Cmd
	stderr bytes.Buffer
	logger *logging.Logger
	source string
	mu     sync.Mutex
	logErr error
}

func newLoggedCapturedExecCommand(cmd *exec.Cmd, logger *logging.Logger) *capturedExecCommand {
	wrapped := &capturedExecCommand{
		cmd:    cmd,
		logger: logger,
		source: filepath.Base(cmd.Path),
	}
	wrapped.cmd.Stderr = io.MultiWriter(&wrapped.stderr, logChunkWriter{
		level:  logging.LevelError,
		source: wrapped.source,
		stream: "stderr",
		logger: logger,
		onError: func(err error) {
			wrapped.addLogError(err)
		},
	})
	return wrapped
}

func (c *capturedExecCommand) SetStdin(r io.Reader) {
	if c.cmd.Stdin == nil {
		c.cmd.Stdin = r
	}
}

func (c *capturedExecCommand) SetStdout(w io.Writer) {
	if c.cmd.Stdout == nil {
		if c.logger == nil {
			c.cmd.Stdout = w
			return
		}
		c.cmd.Stdout = io.MultiWriter(w, logChunkWriter{
			level:  logging.LevelInfo,
			source: c.source,
			stream: "stdout",
			logger: c.logger,
			onError: func(err error) {
				c.addLogError(err)
			},
		})
	}
}

func (c *capturedExecCommand) SetStderr(io.Writer) {
	if c.cmd.Stderr == nil {
		c.cmd.Stderr = &c.stderr
	}
}

func (c *capturedExecCommand) Run() error {
	if c.logger != nil {
		if err := c.logger.Write(logging.LevelInfo, c.source, "started"); err != nil {
			return err
		}
	}
	err := c.cmd.Run()
	if err == nil {
		if c.logger != nil {
			c.addLogError(c.logger.Write(logging.LevelInfo, c.source, "completed"))
		}
		return c.loggingError()
	}
	output := strings.TrimSpace(c.stderr.String())
	if c.logger != nil {
		message := "failed: " + err.Error()
		if output != "" {
			message += ": " + output
		}
		c.addLogError(c.logger.Write(logging.LevelError, c.source, message))
	}
	if output == "" {
		return errors.Join(err, c.loggingError())
	}
	return errors.Join(fmt.Errorf("%w: %s", err, output), c.loggingError())
}

func (c *capturedExecCommand) addLogError(err error) {
	if err == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logErr = errors.Join(c.logErr, err)
}

func (c *capturedExecCommand) loggingError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.logErr
}

type logChunkWriter struct {
	level   logging.Level
	source  string
	stream  string
	logger  *logging.Logger
	onError func(error)
}

func (w logChunkWriter) Write(data []byte) (int, error) {
	content := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		if err := w.logger.Write(w.level, w.source, w.stream+": "+strings.TrimSuffix(line, "\r")); err != nil {
			w.onError(err)
		}
	}
	return len(data), nil
}

func EnterFailureCmd(cellName string, err error) tea.Cmd {
	return func() tea.Msg {
		return enterResultMsg{name: cellName, err: err}
	}
}

func ForkResultCmd(err error) tea.Cmd {
	return func() tea.Msg {
		return forkResultMsg{err: err}
	}
}

type deleteResultMsg struct {
	name string
	err  error
}

// markProgressResultMsg reports the result of the merged or abandoned key.
// progress is the progress the Cell has when err is nil.
type markProgressResultMsg struct {
	name     string
	progress string
	err      error
}

type refreshMsg struct{}

type logEntryMsg logging.Entry
