package output

import (
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/query"
)

const listHeader = "CELL\tTEMPLATE\tCREATION\tSTATUS\tPROGRESS\tPR\tFAILED_STAGE\tLAST_ERROR\n"

func outputCell(issue string, templateName string, note string) query.CellView {
	label := note
	if label == "" {
		label = issue
	}
	return query.CellView{Name: issue, Label: label, Template: templateName, Status: "ready", Progress: "in_progress", CreationStatus: "ready"}
}

func TestFormatCellListはNameとTemplateを表で出力する(t *testing.T) {
	cells := []query.CellView{
		outputCell("123", "default", ""),
		outputCell("456", "webapp", ""),
	}

	got := FormatCellList(cells)
	want := listHeader +
		"123\tdefault\tready\tready\tin_progress\t-\t-\t-\n" +
		"456\twebapp\tready\tready\tin_progress\t-\t-\t-\n"

	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestFormatCellListは空一覧でもヘッダーを出力する(t *testing.T) {
	if got := FormatCellList(nil); got != listHeader {
		t.Fatalf("output = %q, want %q", got, listHeader)
	}
	if got := FormatCellList([]query.CellView{}); got != listHeader {
		t.Fatalf("output = %q, want %q", got, listHeader)
	}
}

func TestFormatCellListはFailed工程と単一行に整形したErrorを出力する(t *testing.T) {
	cell := outputCell("123", "webapp", "")
	cell.CreationStatus, cell.FailedStage, cell.LastError = "failed", "containers", "docker failed\nport already used\ttry another"

	got := FormatCellList([]query.CellView{cell})
	if !strings.Contains(got, "failed\tready\tin_progress\t-\tcontainers\tdocker failed port already used try another") {
		t.Fatalf("output = %q", got)
	}
}

func TestFormatCellListは長いErrorを120文字に切り詰め空のErrorはハイフンにする(t *testing.T) {
	long := outputCell("123", "webapp", "")
	long.CreationStatus, long.FailedStage, long.LastError = "failed", "workspace", strings.Repeat("あ", 130)
	empty := outputCell("456", "webapp", "")
	empty.CreationStatus, empty.FailedStage = "failed", "source"

	lines := strings.Split(strings.TrimSuffix(FormatCellList([]query.CellView{long, empty}), "\n"), "\n")
	if want := "123\twebapp\tfailed\tready\tin_progress\t-\tworkspace\t" + strings.Repeat("あ", 119) + "…"; lines[1] != want {
		t.Fatalf("line = %q, want %q", lines[1], want)
	}
	if want := "456\twebapp\tfailed\tready\tin_progress\t-\tsource\t-"; lines[2] != want {
		t.Fatalf("line = %q, want %q", lines[2], want)
	}
}

func TestFormatCellListはPendingと進み具合と作成中を出力し失敗以外の段階は出さない(t *testing.T) {
	cell := outputCell("123", "webapp", "")
	cell.Status, cell.Progress, cell.CreationStatus = "pending", "merged", "creating"
	unknown := outputCell("456", "webapp", "")
	unknown.Status, unknown.Progress = "", ""

	got := FormatCellList([]query.CellView{cell, unknown})
	want := listHeader +
		"123\twebapp\tcreating\tpending\tmerged\t-\t-\t-\n" +
		"456\twebapp\tready\tready\t-\t-\t-\t-\n"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestFormatCellListはNoteをNameより優先する(t *testing.T) {
	cells := []query.CellView{
		outputCell("123", "default", "PostgreSQL案"),
		outputCell("456", "webapp", ""),
	}

	got := FormatCellList(cells)
	want := listHeader +
		"PostgreSQL案\tdefault\tready\tready\tin_progress\t-\t-\t-\n" +
		"456\twebapp\tready\tready\tin_progress\t-\t-\t-\n"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestFormatCellListはレビュー中のPR番号を出力する(t *testing.T) {
	cell := outputCell("123", "default", "")
	cell.Progress, cell.PullRequests = "in_review", []string{"180", "181"}

	if got, want := FormatCellList([]query.CellView{cell}), listHeader+"123\tdefault\tready\tready\tin_review\t#180,#181\t-\t-\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestFormatUndecidedReviewsは人が選ぶコマンドを出す(t *testing.T) {
	got := FormatUndecidedReviews([]string{"123"})
	if !strings.Contains(got, "WARNING: cell 123") || !strings.Contains(got, "paracell merged 123") || !strings.Contains(got, "paracell abandon 123") {
		t.Fatalf("output = %q", got)
	}
	if FormatUndecidedReviews(nil) != "" {
		t.Fatal("no warning expected")
	}
}
