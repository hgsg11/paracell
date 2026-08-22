package view

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hgsg11/paracell/internal/adapter/logging"
	"github.com/hgsg11/paracell/internal/domain"
)

var errTestReload = errors.New("reload failed")
var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestModelはjで選択を下げる(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
		{ID: "cell-2", Name: "456", Template: "webapp"},
	})

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := next.(Model)
	if got.Selected != 1 {
		t.Fatalf("selected = %d, want %d", got.Selected, 1)
	}
}

func TestModelはkで選択を上げる(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
		{ID: "cell-2", Name: "456", Template: "webapp"},
	})
	model.Selected = 1

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	got := next.(Model)
	if got.Selected != 0 {
		t.Fatalf("selected = %d, want %d", got.Selected, 0)
	}
}

func TestModelは境界で選択を超えない(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	})

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := next.(Model)
	if got.Selected != 0 {
		t.Fatalf("selected = %d, want %d", got.Selected, 0)
	}
}

func TestModelViewは2ペインでTemplateとCellを分離する(t *testing.T) {
	model := NewModel([]domain.Cell{
		{
			ID:       "cell-1",
			Name:     "123",
			Template: "default",
			Issue:    "123",
			Base:     "main",
			Branch:   "feat/123",
			Session:  domain.Session{Name: "paracell-123"},
		},
	})
	model.Templates = []string{"default", "webapp"}

	got := model.View()

	if !strings.Contains(got, "paracell / cells") {
		t.Fatalf("header missing focus line: %q", got)
	}
	if !strings.Contains(got, "default") {
		t.Fatalf("template name not shown: %q", got)
	}
	if !strings.Contains(got, "webapp") {
		t.Fatalf("second template name not shown: %q", got)
	}
}

func TestModelViewはCell一覧にNameTemplateDoneStatusを表示する(t *testing.T) {
	cell := domain.Cell{ID: "cell-1", Name: "123", Template: "default"}
	if err := cell.SetStatus(domain.Pending); err != nil {
		t.Fatalf("SetStatus failed: %v", err)
	}
	model := NewModel([]domain.Cell{cell})
	model.CurrentCell = "123"

	got := model.View()

	if !strings.Contains(got, "* 123") {
		t.Fatalf("current cell marker not shown at row start: %q", got)
	}
	if strings.Contains(got, "> 123") {
		t.Fatalf("legacy selection marker should not appear: %q", got)
	}
	if !strings.Contains(got, "default") {
		t.Fatal("template not shown")
	}
	if !strings.Contains(got, "[ ]") {
		t.Fatal("done checkbox not shown")
	}
	if !strings.Contains(got, "[ ]  ..") {
		t.Fatalf("pending status not shown at row end: %q", got)
	}
}

func TestModelViewは選択中Cell行をReverse表示する(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
		{ID: "cell-2", Name: "456", Template: "webapp"},
	})

	got := model.View()

	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("selected row should contain ansi style: %q", got)
	}
	if !strings.Contains(got, " 123") {
		t.Fatalf("selected row content missing: %q", got)
	}
	if strings.Contains(got, "> 123") {
		t.Fatalf("legacy selection marker should not appear: %q", got)
	}
}

func TestModelViewは現在のCellにだけ中点マーカーを表示する(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
		{ID: "cell-2", Name: "456", Template: "webapp"},
	})
	model.CurrentCell = "456"

	got := model.View()

	if !strings.Contains(got, "* 456") {
		t.Fatalf("current cell marker missing: %q", got)
	}
	if strings.Contains(got, "* 123") {
		t.Fatalf("marker should not appear on other rows: %q", got)
	}
}

func TestModelViewはCell名とNoteを併記してStatusを維持する(t *testing.T) {
	cell := domain.Cell{ID: "cell-1", Name: "123", Note: "API実装中", Template: "default"}
	if err := cell.SetStatus(domain.Pending); err != nil {
		t.Fatal(err)
	}
	model := NewModel([]domain.Cell{cell})

	got := model.View()
	if !strings.Contains(got, "123 | API実装中") {
		t.Fatalf("name and note missing: %q", got)
	}
	if !strings.Contains(got, "[ ]  ..") {
		t.Fatalf("pending status missing: %q", got)
	}
}

func TestModelViewは長いTemplateを省略表示する(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "very-long-template-name"},
	})

	got := model.View()

	if !strings.Contains(got, "very-long-tem...") {
		t.Fatalf("template should be ellipsized: %q", got)
	}
	if strings.Contains(got, "very-long-template-name") {
		t.Fatalf("full template should not be shown: %q", got)
	}
}

func TestModelViewはTemplate一覧の長い名前を省略表示する(t *testing.T) {
	model := NewModel(nil, []string{"very-long-template-name"})
	model.Width = 65
	model.Focus = FocusTemplates

	got := model.View()
	if !strings.Contains(got, "very-long-te...") {
		t.Fatalf("template list name should be ellipsized: %q", got)
	}
	if strings.Contains(got, "very-long-template-name") {
		t.Fatalf("full template list name should not be shown: %q", got)
	}
}

func TestModelViewは長いIssueを省略してPendingStatusを表示する(t *testing.T) {
	cell := domain.Cell{ID: "cell-1", Name: "very-long-issue-name-12345", Template: "default"}
	if err := cell.SetStatus(domain.Pending); err != nil {
		t.Fatalf("SetStatus failed: %v", err)
	}
	model := NewModel([]domain.Cell{cell})
	model.Width = 80

	got := model.View()
	if !strings.Contains(got, "very-long-issue-n...") {
		t.Fatalf("issue should be ellipsized: %q", got)
	}
	if strings.Contains(got, cell.Name) {
		t.Fatalf("full issue should not be shown: %q", got)
	}
	if !strings.Contains(got, "[ ]  ..") {
		t.Fatalf("pending status should remain visible: %q", got)
	}
}

func TestModelViewはReadyでStatusを表示しない(t *testing.T) {
	cell := domain.Cell{ID: "cell-1", Name: "123", Template: "default"}
	if err := cell.SetStatus(domain.Ready); err != nil {
		t.Fatalf("SetStatus failed: %v", err)
	}
	model := NewModel([]domain.Cell{cell})

	got := model.View()

	if strings.Contains(got, "✓") {
		t.Fatalf("ready status should be blank: %q", got)
	}
	if !strings.Contains(got, "default   [ ]") {
		t.Fatalf("ready row layout unexpected: %q", got)
	}
}

func TestModelViewはCell列ヘッダーを表示しない(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	})

	got := model.View()

	if strings.Contains(got, "NAME") || strings.Contains(got, "STATUS") || strings.Contains(got, "DONE") {
		t.Fatalf("view = %q, should not render cell column headers", got)
	}
}

func TestModelViewはCells見出しを表示しない(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	})

	got := model.View()

	if strings.Contains(got, "Cells") {
		t.Fatalf("view = %q, should not render Cells heading", got)
	}
}

func TestModelViewはTemplates見出しを表示しない(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	}, []string{"default", "planning"})

	got := model.View()

	if strings.Contains(got, "Templates") {
		t.Fatalf("view = %q, should not render Templates heading", got)
	}
}

func TestModelViewはSelectedセクションを表示しない(t *testing.T) {
	model := NewModel([]domain.Cell{
		{
			ID:       "cell-1",
			Name:     "123",
			Template: "default",
			Issue:    "123",
			Base:     "main",
			Branch:   "feat/123",
		},
	})

	got := model.View()

	if strings.Contains(got, "Selected") {
		t.Fatalf("view = %q, should not render Selected section", got)
	}
}

func TestModelViewは選択中GoRoot行をReverse表示する(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	})
	model.Focus = FocusExit

	got := model.View()

	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("selected exit row should contain ansi style: %q", got)
	}
	if !strings.Contains(got, "go root") {
		t.Fatalf("selected go root row content missing: %q", got)
	}
	if strings.Contains(got, "> go root") {
		t.Fatalf("legacy selection marker should not appear: %q", got)
	}
}

func TestModelViewはIssue入力用の行をGoRootの直前に常設する(t *testing.T) {
	model := NewModel(
		[]domain.Cell{{ID: "cell-1", Name: "123", Template: "default"}},
		[]string{"default", "planning"},
	)
	model.Focus = FocusTemplates

	got := model.View()
	plain := stripANSI(got)
	lines := strings.Split(plain, "\n")

	if !strings.Contains(got, "planning") {
		t.Fatalf("template list missing: %q", got)
	}
	goRootIndex := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "go root" {
			goRootIndex = i
			break
		}
	}
	if goRootIndex < 1 || strings.TrimSpace(lines[goRootIndex-1]) != "" {
		t.Fatalf("blank issue row should be placed directly above go root: %q", got)
	}
	if strings.Contains(lines[goRootIndex-1], " │ ") {
		t.Fatalf("issue row should be outside template pane: %q", got)
	}
}

func TestModelViewは全体65列の最大幅で描画する(t *testing.T) {
	model := NewModel(
		[]domain.Cell{{ID: "cell-1", Name: "123", Template: "default"}},
		[]string{"default", "planning"},
	)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 10})

	lines := strings.Split(strings.TrimSuffix(updated.(Model).View(), "\n"), "\n")
	// header + spacer + 2 content rows + issue input + go root + status
	if len(lines) != 7 {
		t.Fatalf("line count = %d, want 7: %q", len(lines), updated.(Model).View())
	}
	for i, line := range lines[2:4] {
		plain := stripANSI(line)
		columns := strings.SplitN(plain, " │ ", 2)
		if len(columns) != 2 {
			t.Fatalf("pane row %d has no separator: %q", i, line)
		}
		if lipgloss.Width(columns[0]) != 15 || lipgloss.Width(columns[1]) != 47 {
			t.Fatalf("pane row %d widths = (%d, %d), want (15, 47): %q", i, lipgloss.Width(columns[0]), lipgloss.Width(columns[1]), line)
		}
	}
}

func TestModelViewは選択中のCell行全体をReverse表示する(t *testing.T) {
	model := NewModel([]domain.Cell{{ID: "cell-1", Name: "123", Template: "default"}}, []string{"default"})
	model.Width = 80

	row := strings.Split(model.View(), "\n")[2]
	columns := strings.SplitN(row, " │ ", 2)
	if len(columns) != 2 {
		t.Fatalf("separator missing: %q", row)
	}
	if !strings.HasPrefix(columns[1], "\x1b[7m") || !strings.HasSuffix(columns[1], "\x1b[0m") {
		t.Fatalf("cell pane row is not fully wrapped in reverse style: %q", columns[1])
	}
	if lipgloss.Width(columns[1]) != 47 {
		t.Fatalf("selected cell row width = %d, want 47: %q", lipgloss.Width(columns[1]), columns[1])
	}
}

func TestModelViewはTemplateとGoRootも選択行全体をReverse表示する(t *testing.T) {
	model := NewModel(nil, []string{"default"})
	model.Width = 80
	model.Focus = FocusTemplates

	templateRow := strings.Split(model.View(), "\n")[2]
	templateColumn := strings.SplitN(templateRow, " │ ", 2)[0]
	if !strings.HasPrefix(templateColumn, "\x1b[7m") || !strings.HasSuffix(templateColumn, "\x1b[0m") || lipgloss.Width(templateColumn) != 15 {
		t.Fatalf("template row is not fully reversed: %q", templateColumn)
	}

	model.Focus = FocusExit
	lines := strings.Split(model.View(), "\n")
	goRootRow := lines[len(lines)-3]
	if !strings.HasPrefix(goRootRow, "\x1b[7m") || !strings.HasSuffix(goRootRow, "\x1b[0m") || lipgloss.Width(goRootRow) != 65 {
		t.Fatalf("go root row is not fully reversed: %q", goRootRow)
	}
}

func TestModelViewはHeaderと一覧の間に空行を表示する(t *testing.T) {
	model := NewModel([]domain.Cell{{ID: "cell-1", Name: "123", Template: "default"}}, []string{"default"})
	model.Width = 80

	lines := strings.Split(model.View(), "\n")
	if lines[0] != "paracell / cells" || lines[1] != "" {
		t.Fatalf("header spacer missing: %q", model.View())
	}
}

func TestModelViewは80列未満でもTemplateとCellを左右に配置する(t *testing.T) {
	model := NewModel(
		[]domain.Cell{{ID: "cell-1", Name: "123", Template: "default"}},
		[]string{"default", "planning"},
	)
	model.Width = 79

	got := model.View()
	plain := stripANSI(got)
	if !strings.Contains(plain, " │ ") {
		t.Fatalf("narrow layout should keep the side-by-side separator: %q", got)
	}
}

func TestModelViewは表示高を超えて移動しても選択行を表示する(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "one", Template: "default"},
		{ID: "cell-2", Name: "two", Template: "default"},
		{ID: "cell-3", Name: "three", Template: "default"},
	}, []string{"default"})
	model.Width = 40
	model.Height = 5 // one pane row after reserving the issue input line
	model.Selected = 2

	got := model.View()
	if !strings.Contains(got, "\x1b[7m") || !strings.Contains(got, "three") {
		t.Fatalf("selected row should remain visible after scrolling: %q", got)
	}
}

func TestModelはWindowSizeの幅と高さを保持する(t *testing.T) {
	model := NewModel(nil)
	next, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	got := next.(Model)
	if got.Width != 120 || got.Height != 30 {
		t.Fatalf("size = (%d, %d), want (120, 30)", got.Width, got.Height)
	}
}

func TestModelViewはIssue入力中にGoRootの直前へ内容を表示する(t *testing.T) {
	model := NewModel(
		[]domain.Cell{{ID: "cell-1", Name: "123", Template: "default"}},
		[]string{"default", "planning"},
	)
	model.Focus = FocusTemplates
	model.IssueInputActive = true
	model.IssueInput = "456"

	got := model.View()
	plain := stripANSI(got)
	lines := strings.Split(plain, "\n")
	issueIndex := -1
	goRootIndex := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "issue: 456" {
			issueIndex = i
		}
		if strings.TrimSpace(line) == "go root" {
			goRootIndex = i
		}
	}

	if issueIndex < 0 || issueIndex != goRootIndex-1 {
		t.Fatalf("issue input should be shown directly above go root: %q", got)
	}
	if strings.Contains(lines[issueIndex], " │ ") {
		t.Fatalf("issue input should be outside template pane: %q", got)
	}
	if strings.HasSuffix(got, "issue: 456\n") {
		t.Fatalf("issue input should not replace bottom status line: %q", got)
	}
}

func TestJoinSideBySideはANSI付き行でも区切り位置を揃える(t *testing.T) {
	left := []string{"left", renderSelectedLine("> right")}
	right := []string{"A", "B"}

	got := joinSideBySide(left, right, " | ")
	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("line count = %d, want 2", len(lines))
	}

	firstSep := strings.Index(stripANSI(lines[0]), " | ")
	secondSep := strings.Index(stripANSI(lines[1]), " | ")
	if firstSep < 0 || secondSep < 0 {
		t.Fatalf("separator missing: %q", got)
	}
	if firstSep != secondSep {
		t.Fatalf("separator misaligned: first=%d second=%d output=%q", firstSep, secondSep, got)
	}
}

func stripANSI(value string) string {
	return ansiPattern.ReplaceAllString(value, "")
}

func TestModelはqで終了する(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	})

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got := next.(Model)
	if !got.Quitting {
		t.Fatal("Quitting = false, want true")
	}
	if cmd == nil {
		t.Fatal("qで終了コマンドが返らなかった")
	}
	if got.Result.Action != ActionQuit {
		t.Fatalf("action = %q, want %q", got.Result.Action, ActionQuit)
	}
}

func TestModelはspaceで選択中Cellを返す(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
		{ID: "cell-2", Name: "456", Template: "webapp"},
	})
	model.Selected = 1
	model.Enter = func(cell domain.Cell) tea.Cmd {
		return func() tea.Msg { return enterResultMsg{cell: cell, err: nil} }
	}

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeySpace})
	if cmd == nil {
		t.Fatal("spaceでコマンドが返らなかった")
	}
	updated, nextCmd := next.(Model).Update(cmd())
	got := updated.(Model)
	if got.Result.Action != ActionEnter {
		t.Fatalf("action = %q, want %q", got.Result.Action, ActionEnter)
	}
	if got.Result.Cell.Name != "456" {
		t.Fatalf("cell = %#v, want name %q", got.Result.Cell, "456")
	}
	if nextCmd == nil {
		t.Fatal("space成功で終了コマンドが返らなかった")
	}
}

func TestModelはEnterで選択中CellのDoneを切り替える(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	})
	model.MarkDone = func(cell domain.Cell) (domain.Cell, error) {
		if cell.Name != "123" {
			t.Fatalf("mark done cell = %#v, want name %q", cell, "123")
		}
		if err := cell.MarkDone(); err != nil {
			t.Fatalf("MarkDoneでエラーが返った: %v", err)
		}
		return cell, nil
	}
	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enterでコマンドが返らなかった")
	}
	updated, nextCmd := next.(Model).Update(cmd())
	got := updated.(Model)
	if !got.Cells[0].IsDone() {
		t.Fatal("IsDone = false, want true")
	}
	if got.Result.Action != ActionNone {
		t.Fatalf("action = %q, want %q", got.Result.Action, ActionNone)
	}
	if nextCmd != nil {
		t.Fatal("Enterで終了コマンドが返った")
	}
}

func TestModelはddで選択中Cellを削除する(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
		{ID: "cell-2", Name: "456", Template: "webapp"},
	})
	model.Delete = func(cell domain.Cell) error {
		if cell.Name != "123" {
			t.Fatalf("delete cell = %#v, want name %q", cell, "123")
		}
		return nil
	}

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	got := next.(Model)
	if !got.AwaitingDelete {
		t.Fatal("AwaitingDelete = false, want true")
	}
	if cmd != nil {
		t.Fatal("最初のdでコマンドが返った")
	}

	next, cmd = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd == nil {
		t.Fatal("ddでコマンドが返らなかった")
	}
	updated, nextCmd := next.(Model).Update(cmd())
	got = updated.(Model)
	if got.Error != "" {
		t.Fatalf("error = %q, want empty", got.Error)
	}
	if len(got.Cells) != 1 || got.Cells[0].Name != "456" {
		t.Fatalf("cells = %#v, want remaining cell 456", got.Cells)
	}
	if got.Result.Action != ActionDelete {
		t.Fatalf("action = %q, want %q", got.Result.Action, ActionDelete)
	}
	if nextCmd != nil {
		t.Fatal("delete成功で終了コマンドが返った")
	}
}

func TestModelはRefreshでCellのStatusを再読込する(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	})
	model.Reload = func() ([]domain.Cell, error) {
		cell := domain.Cell{ID: "cell-1", Name: "123", Template: "default"}
		if err := cell.SetStatus(domain.Ready); err != nil {
			t.Fatalf("SetStatusでエラーが返った: %v", err)
		}
		return []domain.Cell{cell}, nil
	}

	next, cmd := model.Update(refreshMsg{})
	got := next.(Model)
	if got.Cells[0].Status() != domain.Ready {
		t.Fatalf("Status = %q, want %q", got.Cells[0].Status(), domain.Ready)
	}
	if cmd == nil {
		t.Fatal("refreshで次のポーリングコマンドが返らなかった")
	}
}

func TestModelはRefreshでPendingStatusのアニメーションを進める(t *testing.T) {
	cell := domain.Cell{ID: "cell-1", Name: "123", Template: "default"}
	if err := cell.SetStatus(domain.Pending); err != nil {
		t.Fatalf("SetStatus failed: %v", err)
	}
	model := NewModel([]domain.Cell{cell})

	first := model.View()
	next, _ := model.Update(refreshMsg{})
	second := next.(Model).View()

	if first == second {
		t.Fatalf("view did not animate: first=%q second=%q", first, second)
	}
	if !strings.Contains(first, "[ ]  ..") {
		t.Fatalf("first frame = %q, want initial pending frame", first)
	}
	if !strings.Contains(second, "[ ]  o.") {
		t.Fatalf("second frame = %q, want advanced pending frame", second)
	}
}

func TestModelはRefresh失敗時に次のポーリングを予約しない(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	})
	model.Reload = func() ([]domain.Cell, error) {
		return nil, errTestReload
	}

	next, cmd := model.Update(refreshMsg{})
	got := next.(Model)

	if got.Error != errTestReload.Error() {
		t.Fatalf("error = %q, want %q", got.Error, errTestReload.Error())
	}
	if cmd != nil {
		t.Fatal("refresh失敗時に次のポーリングコマンドが返った")
	}
}

func TestModelはRefresh成功時に既存エラーを保持する(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	})
	model.Error = "attach failed"
	model.Reload = func() ([]domain.Cell, error) {
		return []domain.Cell{{ID: "cell-1", Name: "123", Template: "default"}}, nil
	}

	next, cmd := model.Update(refreshMsg{})
	got := next.(Model)

	if got.Error != "attach failed" {
		t.Fatalf("error = %q, want %q", got.Error, "attach failed")
	}
	if cmd == nil {
		t.Fatal("refresh成功で次のポーリングコマンドが返らなかった")
	}
}

func TestModelViewはエラー行を常に一行分だけ予約する(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	})

	withoutError := model.View()
	model.Error = "attach failed"
	withError := model.View()

	if strings.Count(withoutError, "\n") != strings.Count(withError, "\n") {
		t.Fatalf("line count changed: without=%q with=%q", withoutError, withError)
	}
	if !strings.HasSuffix(withError, "error: attach failed\n") {
		t.Fatalf("view = %q, want error line suffix", withError)
	}
}

func TestModelは移動してもエラー表示を保持する(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
		{ID: "cell-2", Name: "456", Template: "webapp"},
	})
	model.Error = "attach failed"

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := next.(Model)

	if got.Selected != 1 {
		t.Fatalf("selected = %d, want %d", got.Selected, 1)
	}
	if got.Error != "attach failed" {
		t.Fatalf("error = %q, want %q", got.Error, "attach failed")
	}
	if cmd != nil {
		t.Fatal("error保持中の移動でコマンドが返った")
	}
}

func TestModelViewはエラーの改行を潰して幅で切り詰める(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	})
	model.Width = 20
	model.Error = "first line\nsecond line is very long"

	got := model.View()
	if !strings.HasSuffix(got, "error: first line se\n") {
		t.Fatalf("view = %q, want clipped single-line error", got)
	}
}

func TestCapturedExecCommandはStderrを端末へ直結せずエラーへ含める(t *testing.T) {
	cmd := exec.Command("sh", "-c", "echo noisy stderr >&2; exit 7")
	wrapped := newCapturedExecCommand(cmd)
	wrapped.SetStderr(os.Stderr)

	err := wrapped.Run()
	if err == nil {
		t.Fatal("Run error = nil, want error")
	}
	if !strings.Contains(err.Error(), "noisy stderr") {
		t.Fatalf("error = %q, want stderr output included", err.Error())
	}
}

func TestModelViewはヘッダーなしでログを上詰めし長い行と複数行を画面幅で折り返す(t *testing.T) {
	model := NewModel(nil)
	model.Width = 20
	model.Height = 15
	model.Logs = []logging.Entry{{
		Time:    time.Date(2026, 7, 27, 12, 0, 0, 0, time.Local),
		Level:   logging.LevelError,
		Source:  "paracell",
		Content: "first line\nsecond line is long",
	}}

	got := stripANSI(model.View())
	if lineCount := strings.Count(got, "\n"); lineCount != model.Height {
		t.Fatalf("line count = %d, want screen height %d: %q", lineCount, model.Height, got)
	}
	if strings.Contains(got, "logs\n") {
		t.Fatalf("log header should not be rendered: %q", got)
	}
	exitLineStart := strings.LastIndex(got, "\ngo root") + 1
	logAreaStart := exitLineStart + strings.Index(got[exitLineStart:], "\n") + 1
	logArea := got[logAreaStart:]
	if !strings.HasPrefix(logArea, "2026-07-27 12:00:00.") {
		t.Fatalf("log area should start with the first log line: %q", logArea)
	}
	for _, line := range strings.Split(logArea, "\n") {
		if lipgloss.Width(line) > model.Width {
			t.Fatalf("line width = %d, want <= %d: %q", lipgloss.Width(line), model.Width, line)
		}
	}
	if !strings.Contains(got, "second line is long") {
		t.Fatalf("multiline content missing: %q", got)
	}
}

func TestModelViewは折り返し後の最新行へ追従する(t *testing.T) {
	model := NewModel(nil)
	model.Width = 65
	model.Height = 9
	for i := 1; i <= 6; i++ {
		model.Logs = append(model.Logs, logging.Entry{
			Time:    time.Date(2026, 7, 27, 12, 0, i, 0, time.Local),
			Level:   logging.LevelInfo,
			Source:  "git",
			Content: fmt.Sprintf("line-%d", i),
		})
	}

	got := model.View()
	exitLineStart := strings.LastIndex(got, "\ngo root") + 1
	logAreaStart := exitLineStart + strings.Index(got[exitLineStart:], "\n") + 1
	logArea := got[logAreaStart:]
	if strings.Contains(logArea, "line-1") {
		t.Fatalf("old line should be outside log area: %q", logArea)
	}
	if !strings.Contains(logArea, "line-6") {
		t.Fatalf("latest line missing: %q", logArea)
	}
}

func TestModelはParacellエラーを共通ログへ保存する(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".paracell", "logs", "paracell.log")
	logger := logging.New(path)
	model := NewModel(nil)
	model.Logger = logger

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	if got.Error != "no cells available" {
		t.Fatalf("error = %q", got.Error)
	}
	entry := <-logger.Entries()
	if entry.Level != logging.LevelError || entry.Source != "paracell" || entry.Content != "no cells available" {
		t.Fatalf("entry = %#v", entry)
	}
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(path), "paracell-*.log"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("daily log files = %v, want one file", paths)
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "ERROR [paracell] no cells available") {
		t.Fatalf("log = %q", data)
	}
}

func TestLoggedCapturedExecCommandは成功時もstdoutと完了を保存する(t *testing.T) {
	logger := logging.New(filepath.Join(t.TempDir(), "logs", "paracell.log"))
	cmd := exec.Command("sh", "-c", "echo done")
	wrapped := newLoggedCapturedExecCommand(cmd, logger)
	wrapped.SetStdout(io.Discard)

	if err := wrapped.Run(); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	entries := []logging.Entry{<-logger.Entries(), <-logger.Entries(), <-logger.Entries()}
	if entries[0].Content != "started" || entries[1].Content != "stdout: done" || entries[2].Content != "completed" {
		t.Fatalf("entries = %#v", entries)
	}
}

func TestModelはGoRootをCleanできない(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	})
	model.Focus = FocusExit
	model.Delete = func(cell domain.Cell) error {
		t.Fatalf("go rootでdelete handlerが呼ばれた: %#v", cell)
		return nil
	}

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	got := next.(Model)
	if got.AwaitingDelete {
		t.Fatal("AwaitingDelete = true, want false")
	}
}

func TestModelはGoRootをDoneにできない(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	})
	model.Focus = FocusExit
	model.MarkDone = func(cell domain.Cell) (domain.Cell, error) {
		t.Fatalf("go rootでmark done handlerが呼ばれた: %#v", cell)
		return cell, nil
	}

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd != nil {
		t.Fatal("go rootのEnterでコマンドが返った")
	}
	got := next.(Model)
	if got.Error != "go root cannot be marked done" {
		t.Fatalf("error = %q, want %q", got.Error, "go root cannot be marked done")
	}
}

func TestModelはdのあと別キーなら削除待機を解除する(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
		{ID: "cell-2", Name: "456", Template: "webapp"},
	})

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd != nil {
		t.Fatal("最初のdでコマンドが返った")
	}
	got := next.(Model)
	if !got.AwaitingDelete {
		t.Fatal("AwaitingDelete = false, want true")
	}

	next, cmd = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmd != nil {
		t.Fatal("dのあとjでコマンドが返った")
	}
	got = next.(Model)
	if got.AwaitingDelete {
		t.Fatal("AwaitingDelete = true, want false")
	}
	if got.Selected != 1 {
		t.Fatalf("selected = %d, want %d", got.Selected, 1)
	}
}

func TestModelはEnterでdone状態のCellを解除する(t *testing.T) {
	model := NewModel([]domain.Cell{
		func() domain.Cell {
			cell := domain.Cell{ID: "cell-1", Name: "123", Template: "default"}
			if err := cell.MarkDone(); err != nil {
				t.Fatalf("MarkDoneでエラーが返った: %v", err)
			}
			return cell
		}(),
	})
	model.MarkDone = func(cell domain.Cell) (domain.Cell, error) {
		if cell.Name != "123" {
			t.Fatalf("toggle cell = %#v, want name %q", cell, "123")
		}
		return domain.Cell{ID: cell.ID, Name: cell.Name, Template: cell.Template}, nil
	}

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enterでコマンドが返らなかった")
	}
	updated, _ := next.(Model).Update(cmd())
	if updated.(Model).Cells[0].IsDone() {
		t.Fatal("IsDone = true, want false")
	}
}

func TestModelはtabでTemplate一覧へフォーカスを切り替える(t *testing.T) {
	model := NewModel(
		[]domain.Cell{{ID: "cell-1", Name: "123", Template: "default"}},
		[]string{"default", "planning"},
	)

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := next.(Model)

	if got.Focus != FocusExit {
		t.Fatalf("focus = %v, want %v", got.Focus, FocusExit)
	}
}

func TestModelはtabでTemplateCellExitを巡回する(t *testing.T) {
	model := NewModel(
		[]domain.Cell{{ID: "cell-1", Name: "123", Template: "default"}},
		[]string{"default", "planning"},
	)

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := next.(Model)
	if got.Focus != FocusExit {
		t.Fatalf("focus = %v, want %v", got.Focus, FocusExit)
	}

	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyTab})
	got = next.(Model)
	if got.Focus != FocusTemplates {
		t.Fatalf("focus = %v, want %v", got.Focus, FocusTemplates)
	}

	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyTab})
	got = next.(Model)
	if got.Focus != FocusCells {
		t.Fatalf("focus = %v, want %v", got.Focus, FocusCells)
	}
}

func TestModelはExitParacellでjk移動しない(t *testing.T) {
	model := NewModel(
		[]domain.Cell{{ID: "cell-1", Name: "123", Template: "default"}},
		[]string{"default", "planning"},
	)
	model.Focus = FocusExit

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := next.(Model)
	if got.Focus != FocusExit {
		t.Fatalf("focus = %v, want %v", got.Focus, FocusExit)
	}
	if got.Selected != 0 {
		t.Fatalf("selected = %d, want %d", got.Selected, 0)
	}
	if got.TemplateSelected != 0 {
		t.Fatalf("template selected = %d, want %d", got.TemplateSelected, 0)
	}

	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	got = next.(Model)
	if got.Focus != FocusExit {
		t.Fatalf("focus = %v, want %v", got.Focus, FocusExit)
	}
}

func TestModelはTemplate一覧でyyするとIssue入力モードへ入る(t *testing.T) {
	model := NewModel(nil, []string{"default", "planning"})
	model.Focus = FocusTemplates
	model.TemplateSelected = 1

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	got := next.(Model)
	if !got.AwaitingFork {
		t.Fatal("AwaitingFork = false, want true")
	}
	if cmd != nil {
		t.Fatal("最初のyでコマンドが返った")
	}

	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	got = next.(Model)
	if !got.IssueInputActive {
		t.Fatal("IssueInputActive = false, want true")
	}
	if got.ForkTemplate != "planning" {
		t.Fatalf("ForkTemplate = %q, want %q", got.ForkTemplate, "planning")
	}
}

func TestModelはIssue入力後にEnterでForkHandlerを呼ぶ(t *testing.T) {
	model := NewModel(nil, []string{"default"})
	model.Focus = FocusTemplates
	model.IssueInputActive = true
	model.ForkTemplate = "default"
	model.IssueInput = "123"
	called := false
	model.Fork = func(issue string, template string) tea.Cmd {
		called = true
		if issue != "123" {
			t.Fatalf("issue = %q, want %q", issue, "123")
		}
		if template != "default" {
			t.Fatalf("template = %q, want %q", template, "default")
		}
		return func() tea.Msg {
			return forkResultMsg{cell: domain.Cell{ID: "cell-1", Name: "123", Template: "default"}}
		}
	}

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("issue入力のEnterでforkコマンドが返らなかった")
	}
	if !called {
		t.Fatal("Fork handlerが呼ばれなかった")
	}
	started := next.(Model)
	if started.IssueInputActive {
		t.Fatal("fork開始直後のIssueInputActive = true, want false")
	}
	if started.AwaitingFork {
		t.Fatal("fork開始直後のAwaitingFork = true, want false")
	}
	if started.IssueInput != "" {
		t.Fatalf("fork開始直後のIssueInput = %q, want empty", started.IssueInput)
	}
	if started.ForkTemplate != "" {
		t.Fatalf("fork開始直後のForkTemplate = %q, want empty", started.ForkTemplate)
	}
	if started.ForksInProgress != 1 {
		t.Fatalf("fork開始直後のForksInProgress = %d, want 1", started.ForksInProgress)
	}

	updated, _ := started.Update(cmd())
	got := updated.(Model)
	if got.IssueInputActive {
		t.Fatal("IssueInputActive = true, want false")
	}
	if got.ForksInProgress != 0 {
		t.Fatalf("fork完了後のForksInProgress = %d, want 0", got.ForksInProgress)
	}
}

func TestModelはCell作成中にも別のIssue入力を開始できる(t *testing.T) {
	model := NewModel(nil, []string{"default"})
	model.Focus = FocusTemplates
	model.ForksInProgress = 1

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	got := next.(Model)
	if !got.IssueInputActive {
		t.Fatal("cell作成中のyyでIssue入力モードへ入らなかった")
	}
	if got.ForksInProgress != 1 {
		t.Fatalf("ForksInProgress = %d, want 1", got.ForksInProgress)
	}
}

func TestModelは複数Forkを順不同に完了して実行中件数を追跡する(t *testing.T) {
	model := NewModel(nil, []string{"default"})
	model.Focus = FocusTemplates
	model.Fork = func(issue string, template string) tea.Cmd {
		return func() tea.Msg {
			return forkResultMsg{cell: domain.Cell{ID: "cell-" + issue, Name: issue, Template: template}}
		}
	}
	reloadCalls := 0
	model.Reload = func() ([]domain.Cell, error) {
		reloadCalls++
		return []domain.Cell{{ID: fmt.Sprintf("reloaded-%d", reloadCalls)}}, nil
	}

	model.IssueInputActive = true
	model.AwaitingFork = true
	model.ForkTemplate = "default"
	model.IssueInput = "first"
	next, firstCmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	started := next.(Model)

	next, _ = started.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("second")})
	next, secondCmd := next.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	started = next.(Model)

	if firstCmd == nil || secondCmd == nil {
		t.Fatal("2件のforkコマンドが返らなかった")
	}
	if started.ForksInProgress != 2 {
		t.Fatalf("2件開始後のForksInProgress = %d, want 2", started.ForksInProgress)
	}

	next, _ = started.Update(secondCmd())
	afterSecond := next.(Model)
	if afterSecond.ForksInProgress != 1 {
		t.Fatalf("片方完了後のForksInProgress = %d, want 1", afterSecond.ForksInProgress)
	}
	if reloadCalls != 1 {
		t.Fatalf("片方完了後のreload回数 = %d, want 1", reloadCalls)
	}

	next, _ = afterSecond.Update(firstCmd())
	completed := next.(Model)
	if completed.ForksInProgress != 0 {
		t.Fatalf("全件完了後のForksInProgress = %d, want 0", completed.ForksInProgress)
	}
	if reloadCalls != 2 {
		t.Fatalf("全件完了後のreload回数 = %d, want 2", reloadCalls)
	}
}

func TestModelはFork完了時に次のIssue入力を維持する(t *testing.T) {
	model := NewModel(nil, []string{"default"})
	model.Focus = FocusTemplates
	model.ForksInProgress = 1

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("next")})
	next, _ = next.(Model).Update(forkResultMsg{cell: domain.Cell{ID: "cell-1"}})
	got := next.(Model)

	if !got.IssueInputActive || !got.AwaitingFork {
		t.Fatal("fork完了時に次のIssue入力状態が解除された")
	}
	if got.IssueInput != "next" || got.ForkTemplate != "default" {
		t.Fatalf("fork完了後の入力 = (%q, %q), want (%q, %q)", got.IssueInput, got.ForkTemplate, "next", "default")
	}
	if got.ForksInProgress != 0 {
		t.Fatalf("ForksInProgress = %d, want 0", got.ForksInProgress)
	}
}

func TestModelはFork失敗後もIssue入力を閉じたまま作成中状態を解除する(t *testing.T) {
	model := NewModel(nil, []string{"default"})
	model.IssueInputActive = true
	model.AwaitingFork = true
	model.ForkTemplate = "default"
	model.IssueInput = "123"
	model.Fork = func(string, string) tea.Cmd {
		return func() tea.Msg { return forkResultMsg{err: errors.New("fork failed")} }
	}
	reloadCalls := 0
	model.Reload = func() ([]domain.Cell, error) {
		reloadCalls++
		return nil, nil
	}

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("issue入力のEnterでforkコマンドが返らなかった")
	}
	updated, _ := next.(Model).Update(cmd())
	got := updated.(Model)
	if got.ForksInProgress != 0 {
		t.Fatalf("fork失敗後のForksInProgress = %d, want 0", got.ForksInProgress)
	}
	if got.IssueInputActive || got.AwaitingFork || got.IssueInput != "" {
		t.Fatal("fork失敗後にIssue入力状態が残った")
	}
	if got.Error != "fork failed" {
		t.Fatalf("error = %q", got.Error)
	}
	if reloadCalls != 1 {
		t.Fatalf("fork失敗後のreload回数 = %d, want 1", reloadCalls)
	}
}

func TestModelはFork失敗時に他の実行中Forkを維持する(t *testing.T) {
	model := NewModel(nil)
	model.ForksInProgress = 2
	reloadCalls := 0
	model.Reload = func() ([]domain.Cell, error) {
		reloadCalls++
		return nil, nil
	}

	next, _ := model.Update(forkResultMsg{err: errors.New("fork failed")})
	got := next.(Model)

	if got.ForksInProgress != 1 {
		t.Fatalf("失敗後のForksInProgress = %d, want 1", got.ForksInProgress)
	}
	if got.Error != "fork failed" {
		t.Fatalf("error = %q, want %q", got.Error, "fork failed")
	}
	if reloadCalls != 1 {
		t.Fatalf("fork失敗後のreload回数 = %d, want 1", reloadCalls)
	}
}

func TestModelはIssue入力中にEscで入力を破棄する(t *testing.T) {
	model := NewModel(nil, []string{"default"})
	model.IssueInputActive = true
	model.AwaitingFork = true
	model.ForkTemplate = "default"
	model.IssueInput = "123"

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := next.(Model)

	if got.IssueInputActive {
		t.Fatal("IssueInputActive = true, want false")
	}
	if got.AwaitingFork {
		t.Fatal("AwaitingFork = true, want false")
	}
	if got.IssueInput != "" {
		t.Fatalf("IssueInput = %q, want empty", got.IssueInput)
	}
}

func TestModelはIssue入力中に文字入力とBackspaceができる(t *testing.T) {
	model := NewModel(nil, []string{"default"})
	model.IssueInputActive = true

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1', '2', '3'}})
	got := next.(Model)
	if got.IssueInput != "123" {
		t.Fatalf("IssueInput = %q, want %q", got.IssueInput, "123")
	}

	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	got = next.(Model)
	if got.IssueInput != "12" {
		t.Fatalf("IssueInput = %q, want %q", got.IssueInput, "12")
	}
}

func TestModelViewはTemplate一覧とCell一覧を並べて表示する(t *testing.T) {
	model := NewModel(
		[]domain.Cell{{ID: "cell-1", Name: "123", Template: "default"}},
		[]string{"default", "planning"},
	)

	got := model.View()

	if !strings.Contains(got, "paracell / cells") {
		t.Fatalf("view = %q, want compact header", got)
	}
	if !strings.Contains(got, "default") || !strings.Contains(got, "planning") {
		t.Fatalf("view = %q, want template names", got)
	}
	if !strings.Contains(got, "│") {
		t.Fatalf("view = %q, want split separator", got)
	}
}
func TestModelはlでTemplateからCellsへフォーカスを移す(t *testing.T) {
	model := NewModel(
		[]domain.Cell{{ID: "cell-1", Name: "123", Template: "default"}},
		[]string{"default", "planning"},
	)
	model.Focus = FocusTemplates

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	got := next.(Model)

	if got.Focus != FocusCells {
		t.Fatalf("focus = %v, want %v", got.Focus, FocusCells)
	}
}

func TestModelはhでCellsからTemplateへフォーカスを移す(t *testing.T) {
	model := NewModel(
		[]domain.Cell{{ID: "cell-1", Name: "123", Template: "default"}},
		[]string{"default", "planning"},
	)
	model.Focus = FocusCells

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	got := next.(Model)

	if got.Focus != FocusTemplates {
		t.Fatalf("focus = %v, want %v", got.Focus, FocusTemplates)
	}
}

func TestModelはlでCells端からExitParacellへフォーカスを移す(t *testing.T) {
	model := NewModel(
		[]domain.Cell{{ID: "cell-1", Name: "123", Template: "default"}},
		[]string{"default", "planning"},
	)
	model.Focus = FocusCells

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	got := next.(Model)

	if got.Focus != FocusExit {
		t.Fatalf("focus = %v, want %v", got.Focus, FocusExit)
	}
}

func TestModelはhでTemplates端からExitParacellへフォーカスを移す(t *testing.T) {
	model := NewModel(
		[]domain.Cell{{ID: "cell-1", Name: "123", Template: "default"}},
		[]string{"default", "planning"},
	)
	model.Focus = FocusTemplates

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	got := next.(Model)

	if got.Focus != FocusExit {
		t.Fatalf("focus = %v, want %v", got.Focus, FocusExit)
	}
}
