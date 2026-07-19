package view

import (
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

func TestModelViewはIssue入力用の行をTemplate一覧の下に常設する(t *testing.T) {
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
	planningIndex := -1
	blankIndex := -1
	for i, line := range lines {
		columns := strings.SplitN(line, " │ ", 2)
		if len(columns) != 2 {
			continue
		}
		if strings.TrimSpace(columns[0]) == "planning" {
			planningIndex = i
		}
		if planningIndex >= 0 && i > planningIndex && strings.TrimSpace(columns[0]) == "" {
			blankIndex = i
			break
		}
	}
	if planningIndex < 0 || blankIndex != planningIndex+1 {
		t.Fatalf("blank issue row should be placed directly under templates: %q", got)
	}
}

func TestModelViewは内容量に合わせて左右を半分ずつ同じ高さで描画する(t *testing.T) {
	model := NewModel(
		[]domain.Cell{{ID: "cell-1", Name: "123", Template: "default"}},
		[]string{"default", "planning"},
	)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 10})

	lines := strings.Split(strings.TrimSuffix(updated.(Model).View(), "\n"), "\n")
	// header + 3 content rows + go root + status
	if len(lines) != 6 {
		t.Fatalf("line count = %d, want 6: %q", len(lines), updated.(Model).View())
	}
	for i, line := range lines[1:4] {
		plain := stripANSI(line)
		columns := strings.SplitN(plain, " │ ", 2)
		if len(columns) != 2 {
			t.Fatalf("pane row %d has no separator: %q", i, line)
		}
		if lipgloss.Width(columns[0]) != 38 || lipgloss.Width(columns[1]) != 39 {
			t.Fatalf("pane row %d widths = (%d, %d), want (38, 39): %q", i, lipgloss.Width(columns[0]), lipgloss.Width(columns[1]), line)
		}
	}
}

func TestModelViewは選択中のCell行全体をReverse表示する(t *testing.T) {
	model := NewModel([]domain.Cell{{ID: "cell-1", Name: "123", Template: "default"}}, []string{"default"})
	model.Width = 40

	row := strings.Split(model.View(), "\n")[1]
	columns := strings.SplitN(row, " │ ", 2)
	if len(columns) != 2 {
		t.Fatalf("separator missing: %q", row)
	}
	if !strings.HasPrefix(columns[1], "\x1b[7m") || !strings.HasSuffix(columns[1], "\x1b[0m") {
		t.Fatalf("cell pane row is not fully wrapped in reverse style: %q", columns[1])
	}
	if lipgloss.Width(columns[1]) != 19 {
		t.Fatalf("selected cell row width = %d, want 19: %q", lipgloss.Width(columns[1]), columns[1])
	}
}

func TestModelViewはTemplateとGoRootも選択行全体をReverse表示する(t *testing.T) {
	model := NewModel(nil, []string{"default"})
	model.Width = 40
	model.Focus = FocusTemplates

	templateRow := strings.Split(model.View(), "\n")[1]
	templateColumn := strings.SplitN(templateRow, " │ ", 2)[0]
	if !strings.HasPrefix(templateColumn, "\x1b[7m") || !strings.HasSuffix(templateColumn, "\x1b[0m") || lipgloss.Width(templateColumn) != 18 {
		t.Fatalf("template row is not fully reversed: %q", templateColumn)
	}

	model.Focus = FocusExit
	lines := strings.Split(model.View(), "\n")
	goRootRow := lines[len(lines)-3]
	if !strings.HasPrefix(goRootRow, "\x1b[7m") || !strings.HasSuffix(goRootRow, "\x1b[0m") || lipgloss.Width(goRootRow) != 40 {
		t.Fatalf("go root row is not fully reversed: %q", goRootRow)
	}
}

func TestModelViewは表示高を超えて移動しても選択行を表示する(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "one", Template: "default"},
		{ID: "cell-2", Name: "two", Template: "default"},
		{ID: "cell-3", Name: "three", Template: "default"},
	}, []string{"default"})
	model.Width = 40
	model.Height = 5 // two pane rows
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

func TestModelViewはIssue入力中にTemplate一覧の下へ内容を表示する(t *testing.T) {
	model := NewModel(
		[]domain.Cell{{ID: "cell-1", Name: "123", Template: "default"}},
		[]string{"default", "planning"},
	)
	model.Focus = FocusTemplates
	model.IssueInputActive = true
	model.IssueInput = "456"

	got := model.View()

	if !strings.Contains(got, "issue: 456") {
		t.Fatalf("issue input should be shown in template pane: %q", got)
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

	updated, _ := next.(Model).Update(cmd())
	got := updated.(Model)
	if got.IssueInputActive {
		t.Fatal("IssueInputActive = true, want false")
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
