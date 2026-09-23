package output

import (
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func outputCell(t *testing.T, issue string, templateName string, note string) domain.Cell {
	t.Helper()
	sourceDriver, _ := domain.NewSourceDriverType("git")
	sessionDriver, _ := domain.NewSessionDriverType("tmux")
	cell, err := domain.NewCell("id-"+issue, issue, "sample", templateName, domain.NewSources(sourceDriver, nil), domain.NewContainers(domain.None, nil), domain.NewSession(sessionDriver, nil), domain.NoNotification)
	if err != nil {
		t.Fatal(err)
	}
	if note != "" {
		err = cell.SetNote(note)
		if err != nil {
			t.Fatal(err)
		}
	}
	return cell
}

func TestFormatCellListはNameとTemplateを表で出力する(t *testing.T) {
	cells := []domain.Cell{
		outputCell(t, "123", "default", ""),
		outputCell(t, "456", "webapp", ""),
	}

	got := FormatCellList(cells)
	want := "CELL\tTEMPLATE\tCREATION\tSTATUS\tDONE\tFAILED_STAGE\tLAST_ERROR\n" +
		"123\tdefault\tready\tready\tfalse\t-\t-\n" +
		"456\twebapp\tready\tready\tfalse\t-\t-\n"

	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestFormatCellListは空一覧でもヘッダーを出力する(t *testing.T) {
	got := FormatCellList(nil)
	want := "CELL\tTEMPLATE\tCREATION\tSTATUS\tDONE\tFAILED_STAGE\tLAST_ERROR\n"

	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestFormatCellListはFailed工程と単一行に整形したErrorを出力する(t *testing.T) {
	cell := outputCell(t, "123", "webapp", "")
	stored := cell.Stored()
	stored.Creation.Status = domain.CreationFailed
	stored.Creation.FailedStage = domain.CreationStageContainers
	stored.Creation.LastError = "docker failed\nport already used\ttry another"
	cell, err := domain.RestoreCell(stored)
	if err != nil {
		t.Fatal(err)
	}

	got := FormatCellList([]domain.Cell{cell})
	if !strings.Contains(got, "failed\tready\tfalse\tcontainers\tdocker failed port already used try another") {
		t.Fatalf("output = %q", got)
	}
}

func TestFormatCellListはNoteをNameより優先する(t *testing.T) {
	cells := []domain.Cell{
		outputCell(t, "123", "default", "PostgreSQL案"),
		outputCell(t, "456", "webapp", ""),
	}

	got := FormatCellList(cells)
	want := "CELL\tTEMPLATE\tCREATION\tSTATUS\tDONE\tFAILED_STAGE\tLAST_ERROR\n" +
		"PostgreSQL案\tdefault\tready\tready\tfalse\t-\t-\n" +
		"456\twebapp\tready\tready\tfalse\t-\t-\n"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
