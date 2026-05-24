package view

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shige1114/paradev/internal/domain"
)

type Action string

const (
	ActionNone   Action = ""
	ActionQuit   Action = "quit"
	ActionEnter  Action = "enter"
	ActionDelete Action = "delete"
)

type Result struct {
	Action Action
	Cell   domain.Cell
}

type Model struct {
	Cells          []domain.Cell
	Selected       int
	Quitting       bool
	AwaitingDelete bool
	Error          string
	Result         Result
	Enter          func(domain.Cell) error
	Delete         func(domain.Cell) error
	MarkDone       func(domain.Cell) (domain.Cell, error)
}

func NewModel(cells []domain.Cell) Model {
	return Model{Cells: append([]domain.Cell(nil), cells...)}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		if m.AwaitingDelete {
			m.AwaitingDelete = false
			if key == "d" {
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
			if m.Selected < len(m.Cells)-1 {
				m.Selected++
			}
		case "k":
			if m.Selected > 0 {
				m.Selected--
			}
		case "d":
			m.AwaitingDelete = true
			return m, nil
		case "enter":
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
			if len(m.Cells) == 0 {
				m.Error = "no cells available"
				return m, nil
			}
			cell := m.Cells[m.Selected]
			enter := m.Enter
			return m, func() tea.Msg {
				if enter == nil {
					return enterResultMsg{cell: cell, err: errors.New("enter handler is not configured")}
				}
				return enterResultMsg{cell: cell, err: enter(cell)}
			}
		case "q":
			m.Quitting = true
			m.Result = Result{Action: ActionQuit}
			return m, tea.Quit
		}
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
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	nameWidth, templateWidth := tableWidths(m.Cells)
	fmt.Fprintf(&b, "  %s  %s  DONE\n", padded("NAME", nameWidth), padded("TEMPLATE", templateWidth))
	if len(m.Cells) == 0 {
		b.WriteString("no cells\n")
		return b.String()
	}
	for i, cell := range m.Cells {
		prefix := " "
		if i == m.Selected {
			prefix = ">"
		}
		done := "[ ]"
		if cell.IsDone() {
			done = "[x]"
		}
		fmt.Fprintf(&b, "%s %s  %s  %s\n", prefix, padded(cell.Name, nameWidth), padded(cell.Template, templateWidth), done)
	}
	if m.Error != "" {
		fmt.Fprintf(&b, "\nerror: %s\n", m.Error)
	}
	return b.String()
}

func tableWidths(cells []domain.Cell) (int, int) {
	nameWidth := lipgloss.Width("NAME")
	templateWidth := lipgloss.Width("TEMPLATE")
	for _, cell := range cells {
		nameWidth = max(nameWidth, lipgloss.Width(cell.Name))
		templateWidth = max(templateWidth, lipgloss.Width(cell.Template))
	}
	return nameWidth, templateWidth
}

func padded(value string, width int) string {
	return lipgloss.NewStyle().Width(width).Render(value)
}

type enterResultMsg struct {
	cell domain.Cell
	err  error
}

type deleteResultMsg struct {
	cell domain.Cell
	err  error
}

type markDoneResultMsg struct {
	cell domain.Cell
	err  error
}
