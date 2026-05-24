package output

import (
	"testing"

	"github.com/shige1114/paradev/internal/domain"
)

func TestFormatCellListはNameとTemplateを表で出力する(t *testing.T) {
	cells := []domain.Cell{
		{Name: "123", Template: "default"},
		{Name: "456", Template: "webapp"},
	}

	got := FormatCellList(cells)
	want := "NAME\tTEMPLATE\n123\tdefault\n456\twebapp\n"

	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestFormatCellListは空一覧でもヘッダーを出力する(t *testing.T) {
	got := FormatCellList(nil)
	want := "NAME\tTEMPLATE\n"

	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
