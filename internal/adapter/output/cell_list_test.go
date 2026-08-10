package output

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestFormatCellListはNameとTemplateを表で出力する(t *testing.T) {
	cells := []domain.Cell{
		{Name: "123", Template: "default"},
		{Name: "456", Template: "webapp"},
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
	cell := domain.Cell{Name: "123", Template: "webapp"}
	cell.BeginCreation("command")
	cell.FailCreation(domain.CreationStageContainers, fmt.Errorf("docker failed\nport already used\ttry another"))

	got := FormatCellList([]domain.Cell{cell})
	if !strings.Contains(got, "failed\tready\tfalse\tcontainers\tdocker failed port already used try another") {
		t.Fatalf("output = %q", got)
	}
}

func TestFormatCellListはNoteをNameより優先する(t *testing.T) {
	cells := []domain.Cell{
		{Name: "123", Note: "PostgreSQL案", Template: "default"},
		{Name: "456", Template: "webapp"},
	}

	got := FormatCellList(cells)
	want := "CELL\tTEMPLATE\tCREATION\tSTATUS\tDONE\tFAILED_STAGE\tLAST_ERROR\n" +
		"PostgreSQL案\tdefault\tready\tready\tfalse\t-\t-\n" +
		"456\twebapp\tready\tready\tfalse\t-\t-\n"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
