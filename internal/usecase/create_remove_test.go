package usecase

import (
	"context"
	"reflect"
	"testing"

	"github.com/shige1114/paradev/internal/domain"
)

func TestCreateCellはCellを作成して外部リソースを順番に作る(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	uc := CreateCellUseCase{
		Config:           ports,
		State:            ports,
		SourceFactory:    ports,
		ContainerFactory: ports,
		SessionFactory:   ports,
		IDs:              fixedIDGenerator{id: "cell-1"},
	}

	cell, err := uc.Execute(ctx, CreateCellInput{Issue: "123", Template: "webapp"})
	if err != nil {
		t.Fatalf("CreateCellでエラーが返った: %v", err)
	}
	if cell.ID != "cell-1" {
		t.Fatalf("cell ID = %q, want %q", cell.ID, "cell-1")
	}
	wantCalls := []string{
		"factory:source:git",
		"factory:container:docker",
		"factory:session:tmux",
		"source:create:123",
		"containers:create:123",
		"session:create:123",
		"state:save:1",
	}
	if !reflect.DeepEqual(ports.calls, wantCalls) {
		t.Fatalf("呼び出し順 = %#v, want %#v", ports.calls, wantCalls)
	}
}

func TestCreateCellは同じIssueがある場合に失敗する(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.cells = []domain.Cell{{ID: "existing", Issue: "123", Name: "123"}}
	uc := CreateCellUseCase{
		Config:           ports,
		State:            ports,
		SourceFactory:    ports,
		ContainerFactory: ports,
		SessionFactory:   ports,
		IDs:              fixedIDGenerator{id: "cell-1"},
	}

	_, err := uc.Execute(ctx, CreateCellInput{Issue: "123", Template: "webapp"})

	if err == nil {
		t.Fatal("同じIssueなのにエラーが返らなかった")
	}
	if len(ports.calls) != 0 {
		t.Fatalf("外部リソースが作成された: %#v", ports.calls)
	}
}

func TestRemoveCellはCellを削除してStateから消す(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.cells = []domain.Cell{
		{ID: "cell-1", Issue: "123", Name: "123"},
		{ID: "cell-2", Issue: "456", Name: "456"},
	}
	uc := RemoveCellUseCase{
		Config:           ports,
		State:            ports,
		SourceFactory:    ports,
		ContainerFactory: ports,
		SessionFactory:   ports,
	}

	err := uc.Execute(ctx, RemoveCellInput{Cell: "123"})
	if err != nil {
		t.Fatalf("RemoveCellでエラーが返った: %v", err)
	}
	wantCalls := []string{
		"factory:session:tmux",
		"factory:container:docker",
		"factory:source:git",
		"session:remove:123",
		"containers:remove:123",
		"source:remove:123",
		"state:save:1",
	}
	if !reflect.DeepEqual(ports.calls, wantCalls) {
		t.Fatalf("呼び出し順 = %#v, want %#v", ports.calls, wantCalls)
	}
	if ports.cells[0].Issue != "456" {
		t.Fatalf("残ったcell issue = %q, want %q", ports.cells[0].Issue, "456")
	}
}

func TestListCellsはStateのCell一覧を返す(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.cells = []domain.Cell{
		{ID: "cell-1", Issue: "123", Name: "123", Template: "default"},
		{ID: "cell-2", Issue: "456", Name: "456", Template: "webapp"},
	}
	uc := ListCellsUseCase{State: ports}

	cells, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("ListCellsでエラーが返った: %v", err)
	}
	if !reflect.DeepEqual(cells, ports.cells) {
		t.Fatalf("cells = %#v, want %#v", cells, ports.cells)
	}
}

type fakePorts struct {
	config domain.Config
	cells  []domain.Cell
	calls  []string
}

func newFakePorts() *fakePorts {
	return &fakePorts{
		config: domain.Config{
			Project: domain.ProjectConfig{Name: "myapp"},
			Providers: domain.ProviderConfig{
				Source:    "git",
				Container: "docker",
				Session:   "tmux",
			},
			Templates: map[string]domain.Template{
				"webapp": {
					Name: "webapp",
					Repository: domain.RepositoryTemplate{
						BranchPrefix: "feat/",
						Base:         "main",
					},
					Containers: domain.ContainerTemplate{
						Services: map[string]domain.ContainerServiceTemplate{
							"web": {SourceContainer: "myapp-web"},
						},
					},
				},
			},
		},
	}
}

func (f *fakePorts) Load(ctx context.Context) (domain.Config, error) {
	return f.config, nil
}

func (f *fakePorts) LoadCells(ctx context.Context) ([]domain.Cell, error) {
	return append([]domain.Cell(nil), f.cells...), nil
}

func (f *fakePorts) SaveCells(ctx context.Context, cells []domain.Cell) error {
	f.cells = append([]domain.Cell(nil), cells...)
	f.calls = append(f.calls, "state:save:"+string(rune('0'+len(cells))))
	return nil
}

func (f *fakePorts) Source(provider domain.ProviderConfig) (SourcePort, error) {
	f.calls = append(f.calls, "factory:source:"+provider.Source)
	return f, nil
}

func (f *fakePorts) Container(provider domain.ProviderConfig) (ContainerPort, error) {
	f.calls = append(f.calls, "factory:container:"+provider.Container)
	return f, nil
}

func (f *fakePorts) Session(provider domain.ProviderConfig) (SessionPort, error) {
	f.calls = append(f.calls, "factory:session:"+provider.Session)
	return f, nil
}

func (f *fakePorts) CreateSource(ctx context.Context, cell domain.Cell) error {
	f.calls = append(f.calls, "source:create:"+cell.Name)
	return nil
}

func (f *fakePorts) RemoveSource(ctx context.Context, cell domain.Cell) error {
	f.calls = append(f.calls, "source:remove:"+cell.Name)
	return nil
}

func (f *fakePorts) CreateContainers(ctx context.Context, cell domain.Cell, template domain.Template) error {
	f.calls = append(f.calls, "containers:create:"+cell.Name)
	return nil
}

func (f *fakePorts) RemoveContainers(ctx context.Context, cell domain.Cell) error {
	f.calls = append(f.calls, "containers:remove:"+cell.Name)
	return nil
}

func (f *fakePorts) CreateSession(ctx context.Context, cell domain.Cell) error {
	f.calls = append(f.calls, "session:create:"+cell.Name)
	return nil
}

func (f *fakePorts) RemoveSession(ctx context.Context, cell domain.Cell) error {
	f.calls = append(f.calls, "session:remove:"+cell.Name)
	return nil
}

func (f *fakePorts) EnterSession(ctx context.Context, cell domain.Cell) error {
	f.calls = append(f.calls, "session:enter:"+cell.Name)
	return nil
}

type fixedIDGenerator struct {
	id string
}

func (g fixedIDGenerator) NewID() string {
	return g.id
}
