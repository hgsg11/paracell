package state

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestStateが存在しない場合は空のCell一覧を返す(t *testing.T) {
	store := JSONCellStateAdapter{Path: filepath.Join(t.TempDir(), ".paracell", "state.json")}

	cells, err := store.LoadCells(context.Background())

	if err != nil {
		t.Fatalf("空state読み込みでエラーが返った: %v", err)
	}
	if len(cells) != 0 {
		t.Fatalf("cells length = %d, want 0", len(cells))
	}
}

func TestCellを保存して読み戻せる(t *testing.T) {
	store := JSONCellStateAdapter{Path: filepath.Join(t.TempDir(), ".paracell", "state.json")}
	cell := domain.Cell{ID: "cell-1", Issue: "123", Name: "123", Template: "webapp"}
	if err := cell.MarkDone(); err != nil {
		t.Fatalf("Cellをdoneにできなかった: %v", err)
	}
	if err := cell.SetStatus(domain.CellStatusReady); err != nil {
		t.Fatalf("Cellをreadyにできなかった: %v", err)
	}

	if err := store.SaveCells(context.Background(), []domain.Cell{cell}); err != nil {
		t.Fatalf("state保存でエラーが返った: %v", err)
	}
	cells, err := store.LoadCells(context.Background())

	if err != nil {
		t.Fatalf("state読み込みでエラーが返った: %v", err)
	}
	if len(cells) != 1 {
		t.Fatalf("cells length = %d, want 1", len(cells))
	}
	if cells[0].ID != "cell-1" {
		t.Fatalf("cell ID = %q, want %q", cells[0].ID, "cell-1")
	}
	if !cells[0].IsDone() {
		t.Fatal("IsDone = false, want true")
	}
	if got := cells[0].Status(); got != domain.CellStatusReady {
		t.Fatalf("Status = %q, want %q", got, domain.CellStatusReady)
	}
}

func TestCellは未設定StatusならReadyとして読み戻せる(t *testing.T) {
	store := JSONCellStateAdapter{Path: filepath.Join(t.TempDir(), ".paracell", "state.json")}
	cell := domain.Cell{ID: "cell-1", Issue: "123", Name: "123", Template: "webapp"}

	if err := store.SaveCells(context.Background(), []domain.Cell{cell}); err != nil {
		t.Fatalf("state保存でエラーが返った: %v", err)
	}
	cells, err := store.LoadCells(context.Background())
	if err != nil {
		t.Fatalf("state読み込みでエラーが返った: %v", err)
	}
	if got := cells[0].Status(); got != domain.CellStatusReady {
		t.Fatalf("Status = %q, want %q", got, domain.CellStatusReady)
	}
}

func TestCellStateはDatabase設定を保存して読み戻せる(t *testing.T) {
	store := JSONCellStateAdapter{Path: filepath.Join(t.TempDir(), ".paracell", "state.json")}
	cell := domain.Cell{
		ID:       "cell-1",
		Issue:    "123",
		Name:     "123",
		Template: "webapp",
		Base:     "current",
		Containers: domain.Containers{
			Services: map[string]domain.CellContainer{
				"db": {
					ContainerName:   "paracell-myapp-123-db",
					SourceContainer: "myapp-db",
					Database: &domain.DatabaseConfig{
						System:    "mysql",
						CopyMode:  "schema",
						InitFiles: []string{"docker/mysql/init/001-users.sql"},
					},
				},
				"web": {
					ContainerName:   "paracell-myapp-123-web",
					SourceContainer: "myapp-web",
					VolumeMode:      "copy",
				},
			},
		},
	}

	if err := store.SaveCells(context.Background(), []domain.Cell{cell}); err != nil {
		t.Fatalf("state保存でエラーが返った: %v", err)
	}
	cells, err := store.LoadCells(context.Background())
	if err != nil {
		t.Fatalf("state読み込みでエラーが返った: %v", err)
	}
	if len(cells) != 1 {
		t.Fatalf("cells length = %d, want 1", len(cells))
	}
	if got := cells[0].Base; got != "current" {
		t.Fatalf("base = %q, want %q", got, "current")
	}
	got := cells[0].Containers.Services["db"].Database
	want := &domain.DatabaseConfig{
		System:    "mysql",
		CopyMode:  "schema",
		InitFiles: []string{"docker/mysql/init/001-users.sql"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("database = %#v, want %#v", got, want)
	}
	if got := cells[0].Containers.Services["web"].VolumeMode; got != "copy" {
		t.Fatalf("volumeMode = %q, want %q", got, "copy")
	}
}
