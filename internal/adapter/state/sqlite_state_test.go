package state

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestSQLiteStateはSourcesとContainerModeを保存する(t *testing.T) {
	adapter := SQLiteCellStateAdapter{Path: filepath.Join(t.TempDir(), "state.db")}
	want := []domain.Cell{{
		ID: "id", Issue: "42", Name: "42", Template: "feat",
		Sources: []domain.Source{{TemplatePath: ".", Path: "source", Base: "main", Branch: "feat/42"}},
		Containers: domain.Containers{Network: "cell-42", Services: map[string]domain.CellContainer{
			"db": {ContainerName: "db", SourceContainer: "db", Mode: domain.Dependency},
		}},
		Session: domain.Session{Name: "session"},
	}}
	if err := adapter.SaveCells(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	got, err := adapter.LoadCells(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got[0].Sources, want[0].Sources) || got[0].Containers.Services["db"].Mode != domain.Dependency {
		t.Fatalf("got = %#v", got)
	}
}
