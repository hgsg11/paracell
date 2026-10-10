package output

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
	"github.com/hgsg11/paracell/internal/usecase"
)

func outputCells(t *testing.T, specs ...[3]string) usecase.CellSet {
	t.Helper()
	set := usecase.NewCellSet(nil, nil, nil, nil)
	for _, spec := range specs {
		issue, templateName, note := spec[0], spec[1], spec[2]
		group, err := domain.NewCellGroup("group-id-"+issue, issue, "sample", templateName, domain.Git, domain.None, domain.NoNotification)
		if err != nil {
			t.Fatal(err)
		}
		if note != "" {
			if err := group.SetNote(note); err != nil {
				t.Fatal(err)
			}
		}
		cell, err := domain.NewCommanderCell("id-"+issue, group.ID, domain.NewWorkspace(domain.Tmux, nil))
		if err != nil {
			t.Fatal(err)
		}
		set.Commanders = append(set.Commanders, cell)
		set.Groups = append(set.Groups, group)
	}
	return set
}

func TestFormatCellListはNameとTemplateを表で出力する(t *testing.T) {
	got := FormatCellList(outputCells(t, [3]string{"123", "default", ""}, [3]string{"456", "webapp", ""}))
	want := "CELL\tTEMPLATE\tCREATION\tSTATUS\tDONE\tFAILED_STAGE\tLAST_ERROR\n" +
		"123\tdefault\tready\tready\tfalse\t-\t-\n" +
		"456\twebapp\tready\tready\tfalse\t-\t-\n"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestFormatCellListは空一覧でもヘッダーを出力する(t *testing.T) {
	got := FormatCellList(usecase.CellSet{})
	want := "CELL\tTEMPLATE\tCREATION\tSTATUS\tDONE\tFAILED_STAGE\tLAST_ERROR\n"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestFormatCellListはFailed工程と単一行に整形したErrorを出力する(t *testing.T) {
	set := outputCells(t, [3]string{"123", "webapp", ""})
	set.Groups[0].BeginCreation()
	set.Groups[0].FailCreation(domain.CreationStageContainers, fmt.Errorf("docker failed\nport already used\ttry another"))
	got := FormatCellList(set)
	if !strings.Contains(got, "failed\tready\tfalse\tcontainers\tdocker failed port already used try another") {
		t.Fatalf("output = %q", got)
	}
}

func TestFormatCellListはNoteをNameより優先する(t *testing.T) {
	got := FormatCellList(outputCells(t, [3]string{"123", "default", "PostgreSQL案"}, [3]string{"456", "webapp", ""}))
	want := "CELL\tTEMPLATE\tCREATION\tSTATUS\tDONE\tFAILED_STAGE\tLAST_ERROR\n" +
		"PostgreSQL案\tdefault\tready\tready\tfalse\t-\t-\n" +
		"456\twebapp\tready\tready\tfalse\t-\t-\n"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
