package usecase

import (
	"bytes"
	"context"
	"reflect"
	"testing"
	"text/template"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestForkCellはCellを作成して外部リソースを順番に作る(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	uc := ForkCellUseCase{
		Config:           ports,
		State:            ports,
		SourceFactory:    ports,
		ContainerFactory: ports,
		SessionFactory:   ports,
		Files:            ports,
		IDs:              fixedIDGenerator{id: "cell-1"},
	}

	cell, err := uc.Execute(ctx, ForkCellInput{Issue: "123", Template: "webapp"})
	if err != nil {
		t.Fatalf("ForkCellでエラーが返った: %v", err)
	}
	if cell.ID != "cell-1" {
		t.Fatalf("cell ID = %q, want %q", cell.ID, "cell-1")
	}
	if cell.IsDone() {
		t.Fatal("IsDone = true, want false")
	}
	wantCalls := []string{
		"factory:source:git",
		"factory:container:docker",
		"factory:session:tmux",
		"source:fork:123",
		"files:copy:123:.env,apps/web/.env.local",
		"containers:fork:123",
		"session:fork:123:nvim 123",
		"state:save:1",
	}
	if !reflect.DeepEqual(ports.calls, wantCalls) {
		t.Fatalf("呼び出し順 = %#v, want %#v", ports.calls, wantCalls)
	}
}

func TestForkCellは同じIssueがある場合に失敗する(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.cells = []domain.Cell{{ID: "existing", Issue: "123", Name: "123"}}
	uc := ForkCellUseCase{
		Config:           ports,
		State:            ports,
		SourceFactory:    ports,
		ContainerFactory: ports,
		SessionFactory:   ports,
		Files:            ports,
		IDs:              fixedIDGenerator{id: "cell-1"},
	}

	_, err := uc.Execute(ctx, ForkCellInput{Issue: "123", Template: "webapp"})

	if err == nil {
		t.Fatal("同じIssueなのにエラーが返らなかった")
	}
	if len(ports.calls) != 0 {
		t.Fatalf("外部リソースが作成された: %#v", ports.calls)
	}
}

func TestCleanCellはCellを削除してStateから消す(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.cells = []domain.Cell{
		func() domain.Cell {
			cell := domain.Cell{ID: "cell-1", Issue: "123", Name: "123"}
			if err := cell.MarkDone(); err != nil {
				t.Fatalf("Cellをdoneにできなかった: %v", err)
			}
			return cell
		}(),
		{ID: "cell-2", Issue: "456", Name: "456"},
	}
	uc := CleanCellUseCase{
		Config:           ports,
		State:            ports,
		SourceFactory:    ports,
		ContainerFactory: ports,
		SessionFactory:   ports,
	}

	err := uc.Execute(ctx, CleanCellInput{Cell: "123"})
	if err != nil {
		t.Fatalf("CleanCellでエラーが返った: %v", err)
	}
	wantCalls := []string{
		"factory:session:tmux",
		"factory:container:docker",
		"factory:source:git",
		"session:clean:123",
		"containers:clean:123",
		"source:clean:123",
		"state:save:1",
	}
	if !reflect.DeepEqual(ports.calls, wantCalls) {
		t.Fatalf("呼び出し順 = %#v, want %#v", ports.calls, wantCalls)
	}
	if ports.cells[0].Issue != "456" {
		t.Fatalf("残ったcell issue = %q, want %q", ports.cells[0].Issue, "456")
	}
}

func TestCleanCellはDoneでないCellをCleanしない(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.cells = []domain.Cell{
		{ID: "cell-1", Issue: "123", Name: "123"},
	}
	uc := CleanCellUseCase{
		Config:           ports,
		State:            ports,
		SourceFactory:    ports,
		ContainerFactory: ports,
		SessionFactory:   ports,
	}

	err := uc.Execute(ctx, CleanCellInput{Cell: "123"})

	if err == nil {
		t.Fatal("doneでないcellなのに削除できてしまった")
	}
	if err.Error() != "完了済みではないので消せない" {
		t.Fatalf("error = %q, want %q", err.Error(), "完了済みではないので消せない")
	}
}

func TestMarkCellDoneはStateのCellのDoneを切り替える(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.cells = []domain.Cell{
		{ID: "cell-1", Issue: "123", Name: "123"},
	}

	uc := MarkCellDoneUseCase{State: ports}
	cell, err := uc.Execute(ctx, MarkCellDoneInput{Cell: "123"})
	if err != nil {
		t.Fatalf("MarkCellDoneでエラーが返った: %v", err)
	}
	if !cell.IsDone() {
		t.Fatal("IsDone = false, want true")
	}
	if !ports.cells[0].IsDone() {
		t.Fatal("stateのcellがdoneになっていない")
	}

	cell, err = uc.Execute(ctx, MarkCellDoneInput{Cell: "123"})
	if err != nil {
		t.Fatalf("MarkCellDoneの解除でエラーが返った: %v", err)
	}
	if cell.IsDone() {
		t.Fatal("IsDone = true, want false")
	}
	if ports.cells[0].IsDone() {
		t.Fatal("stateのcellがdoneのままになっている")
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
					Files: []string{".env", "apps/web/.env.local"},
					Containers: domain.ContainerTemplate{
						Services: map[string]domain.ContainerServiceTemplate{
							"web": {SourceContainer: "myapp-web"},
						},
					},
					Session: domain.SessionTemplate{
						Windows: []domain.SessionWindowTemplate{{Name: "editor", Command: "nvim {{.issue}}"}},
					},
				},
			},
		},
	}
}

func (f *fakePorts) Load(ctx context.Context, vars *domain.TemplateVars) (domain.Config, error) {
	_ = ctx
	if vars == nil {
		return f.config, nil
	}
	cfg := f.config
	cfg.Templates = make(map[string]domain.Template, len(f.config.Templates))
	for name, tpl := range f.config.Templates {
		rendered := tpl
		rendered.Session.Windows = make([]domain.SessionWindowTemplate, 0, len(tpl.Session.Windows))
		for _, window := range tpl.Session.Windows {
			command, err := renderString(window.Command, map[string]string{
				"issue": vars.Issue,
				"name":  vars.Name,
			})
			if err != nil {
				return domain.Config{}, err
			}
			rendered.Session.Windows = append(rendered.Session.Windows, domain.SessionWindowTemplate{
				Name:    window.Name,
				Command: command,
			})
		}
		cfg.Templates[name] = rendered
	}
	return cfg, nil
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
	f.calls = append(f.calls, "source:fork:"+cell.Name)
	return nil
}

func (f *fakePorts) CleanSource(ctx context.Context, cell domain.Cell) error {
	f.calls = append(f.calls, "source:clean:"+cell.Name)
	return nil
}

func (f *fakePorts) CopyFiles(ctx context.Context, cell domain.Cell, template domain.Template) error {
	f.calls = append(f.calls, "files:copy:"+cell.Name+":"+joinStrings(template.Files))
	return nil
}

func (f *fakePorts) CreateContainers(ctx context.Context, cell domain.Cell, template domain.Template) error {
	f.calls = append(f.calls, "containers:fork:"+cell.Name)
	return nil
}

func (f *fakePorts) CleanContainers(ctx context.Context, cell domain.Cell) error {
	f.calls = append(f.calls, "containers:clean:"+cell.Name)
	return nil
}

func (f *fakePorts) CreateSession(ctx context.Context, cell domain.Cell) error {
	command := ""
	if len(cell.Session.Windows) > 0 {
		command = ":" + cell.Session.Windows[0].Command
	}
	f.calls = append(f.calls, "session:fork:"+cell.Name+command)
	return nil
}

func (f *fakePorts) CleanSession(ctx context.Context, cell domain.Cell) error {
	f.calls = append(f.calls, "session:clean:"+cell.Name)
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

func joinStrings(values []string) string {
	if len(values) == 0 {
		return ""
	}
	out := values[0]
	for _, value := range values[1:] {
		out += "," + value
	}
	return out
}

func renderString(value string, data map[string]string) (string, error) {
	tmpl, err := template.New("value").Option("missingkey=error").Parse(value)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := tmpl.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}
