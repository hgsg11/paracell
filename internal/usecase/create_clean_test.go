package usecase

import (
	"context"
	"errors"
	"github.com/hgsg11/paracell/internal/adapter/id"
	"reflect"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestForkCellは新しいTemplateからCellを作る(t *testing.T) {
	ports := newFakePorts()
	usecase := newForkCellUseCase(ports)
	cell, err := usecase.Execute(context.Background(), ForkCellInput{Issue: "42", Template: "feat"})
	if err != nil {
		t.Fatal(err)
	}
	if cell.CellGroup.Template != "feat" || len(ports.cells.Targets) != 1 || cell.CellGroup.CreationStatus() != domain.CreationReady {
		t.Fatalf("cell = %#v", cell)
	}
	if got, want := cell.ResourceDrivers(), domain.NewCellDrivers(domain.Git, domain.None, domain.Tmux, domain.NoNotification); got != want {
		t.Fatalf("drivers = %#v, want %#v", got, want)
	}
	wantCalls := []string{
		"factory:source:git",
		"factory:container:none",
		"factory:workspace:tmux",
		"source:create",
		"containers:create",
		"workspace:create",
	}
	if !reflect.DeepEqual(ports.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", ports.calls, wantCalls)
	}
}

func TestForkCellはSource作成失敗時も作成対象をCellに保持する(t *testing.T) {
	ports := newFakePorts()
	ports.createSourceErr = errors.New("create source")

	_, err := newForkCellUseCase(ports).Execute(context.Background(), ForkCellInput{Issue: "42", Template: "feat"})
	if !errors.Is(err, ports.createSourceErr) {
		t.Fatalf("error = %v", err)
	}
	if len(ports.cells.Commanders) != 1 {
		t.Fatalf("commanders = %d", len(ports.cells.Commanders))
	}
	failedStage, _ := ports.cells.Commanders[0].CellGroup.CreationFailure()
	if ports.cells.Commanders[0].CellGroup.CreationStatus() != domain.CreationFailed || failedStage != domain.CreationStageSource {
		t.Fatalf("commander = %#v", ports.cells.Commanders[0])
	}
	for _, resource := range domain.BuildSourceResourcesService(ports.cells.Commanders[0], ports.cells.Targets) {
		if err := ports.CleanSource(context.Background(), resource); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := ports.cleanedSources, []domain.SourceResource{domain.NewSourceResource(".", ".paracell/cells/42/repository/source", "main", "feat/42")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("cleaned sources = %#v, want %#v", got, want)
	}
}

type fakePorts struct {
	config               domain.Templates
	configErr            error
	cells                CellSet
	calls                []string
	updateStatusLabelErr error
	createSourceErr      error
	cleanedSources       []domain.SourceResource
	containerResources   domain.ContainerResources
	workspaceResource    domain.WorkspaceResource
}

func newFakePorts() *fakePorts {
	source, _ := domain.NewSourceTemplate(".", "main", "feat/")
	target, _ := domain.NewTargetCellSpec("repository", &source, nil)
	commander, _ := domain.NewCommanderCellSpec("workspace", domain.NewWorkspaceTemplate(nil))
	template, _ := domain.NewUnresolvedTemplate("feat", "", false, &commander, []domain.TargetCellSpec{target}, nil)
	workspaceDriver, _ := domain.NewWorkspaceDriverType("tmux")
	sourceDriver, _ := domain.NewSourceDriverType("git")
	notificationDriver, _ := domain.NewNotificationDriverType("")
	templates, _ := domain.NewTemplates("myapp", []domain.Template{template}, workspaceDriver, domain.NewContainerDriverType(""), sourceDriver, notificationDriver)
	return &fakePorts{config: templates}
}

func newForkCellUseCase(ports *fakePorts) ForkCellUseCase {
	return ForkCellUseCase{
		Config: ports, Cells: ports, SourceFactory: ports,
		ContainerFactory: ports, WorkspaceFactory: ports, IDs: fixedIDGenerator{id: "cell-1"},
	}
}

func (f *fakePorts) Load(context.Context) (domain.Templates, error) {
	return f.config, f.configErr
}

func (f *fakePorts) LoadCells(context.Context) (CellSet, error) {
	return NewCellSet(f.cells.Commanders, f.cells.Targets, f.cells.Dependencies), nil
}

func (f *fakePorts) UpdateCells(_ context.Context, update func(CellSet) (CellSet, error)) error {
	before := NewCellSet(f.cells.Commanders, f.cells.Targets, f.cells.Dependencies)
	for i := range before.Commanders {
		before.Commanders[i] = before.Commanders[i].Clone()
	}
	cells, err := update(NewCellSet(f.cells.Commanders, f.cells.Targets, f.cells.Dependencies))
	if err != nil {
		return err
	}
	for index := range cells.Commanders {
		for _, previous := range before.Commanders {
			if previous.ID == cells.Commanders[index].ID && !reflect.DeepEqual(previous, cells.Commanders[index]) {
				if err := cells.Commanders[index].AdvanceVersion(); err != nil {
					return err
				}
			}
		}
	}
	f.cells = cells
	return nil
}

func (f *fakePorts) Source(driver domain.SourceDriverType) (SourcePort, error) {
	f.calls = append(f.calls, "factory:source:"+string(driver))
	return f, nil
}

func (f *fakePorts) Container(driver domain.ContainerDriverType) (ContainerPort, error) {
	f.calls = append(f.calls, "factory:container:"+string(driver))
	return f, nil
}

func (f *fakePorts) Workspace(driver domain.WorkspaceDriverType) (WorkspacePort, error) {
	f.calls = append(f.calls, "factory:workspace:"+string(driver))
	return f, nil
}

func (f *fakePorts) CreateSource(context.Context, domain.SourceResource) error {
	f.calls = append(f.calls, "source:create")
	return f.createSourceErr
}
func (f *fakePorts) CleanSource(_ context.Context, resource domain.SourceResource) error {
	f.calls = append(f.calls, "source:clean")
	f.cleanedSources = append(f.cleanedSources, resource)
	return nil
}
func (f *fakePorts) CreateContainers(_ context.Context, resources domain.ContainerResources) (map[string][]string, error) {
	f.calls = append(f.calls, "containers:create")
	f.containerResources = resources
	return map[string][]string{"app": {"original_default"}}, nil
}
func (f *fakePorts) CleanContainers(context.Context, domain.ContainerResources) error {
	f.calls = append(f.calls, "containers:clean")
	return nil
}
func (f *fakePorts) CreateWorkspace(_ context.Context, resource domain.WorkspaceResource) error {
	f.calls = append(f.calls, "workspace:create")
	f.workspaceResource = resource
	return nil
}
func (f *fakePorts) CleanWorkspace(context.Context, domain.WorkspaceResource) error {
	f.calls = append(f.calls, "workspace:clean")
	return nil
}
func (f *fakePorts) PrepareWorkspace(context.Context, domain.WorkspaceResource) error { return nil }
func (f *fakePorts) UpdateStatusLabel(_ context.Context, resource domain.WorkspaceResource) error {
	f.calls = append(f.calls, "workspace:label:"+resource.DisplayLabel)
	return f.updateStatusLabelErr
}
func (f *fakePorts) EnterWorkspace(_ context.Context, resource domain.WorkspaceResource) error {
	f.calls = append(f.calls, "workspace:enter:"+resource.CellName)
	return nil
}
func (f *fakePorts) EnterRootWorkspace(_ context.Context, projectName string) error {
	f.calls = append(f.calls, "workspace:enter-root:"+projectName)
	return nil
}
func (f *fakePorts) ExitWorkspace(context.Context) error {
	f.calls = append(f.calls, "workspace:exit")
	return nil
}

type fixedIDGenerator struct{ id string }

func (g fixedIDGenerator) NewID() string { return g.id }

func newUsecaseTestCell(t *testing.T, id string, issue string, templateName string) domain.CommanderCell {
	t.Helper()
	sourceDriver, _ := domain.NewSourceDriverType("git")
	workspaceDriver, _ := domain.NewWorkspaceDriverType("tmux")
	group, err := domain.NewCellGroup("group-"+id, issue, "myapp", templateName, sourceDriver, domain.None, domain.NoNotification)
	if err != nil {
		t.Fatal(err)
	}
	cell, err := domain.NewCommanderCell(id, &group, domain.NewWorkspace(workspaceDriver, nil))
	if err != nil {
		t.Fatal(err)
	}
	return cell
}

func TestCellGroupsPreserveTemplateLinksAndCleanOnlySelectedGroup(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	source, _ := domain.NewSourceTemplate(".", "main", "feat/")
	app, _ := domain.NewContainerTemplate("app", domain.Target, nil, nil)
	api, _ := domain.NewTargetCellSpec("api", &source, []domain.ContainerTemplate{app})
	web, _ := domain.NewTargetCellSpec("web", &source, nil)
	database, _ := domain.NewDependencyCellSpec("database")
	window, _ := domain.NewWindow("agent", "codex {{.Command}}")
	spec, err := domain.NewCommanderCellSpec("workspace", domain.NewWorkspaceTemplate([]domain.Window{window}))
	if err != nil {
		t.Fatal(err)
	}
	template, _ := domain.NewUnresolvedTemplate("feat", "", false, &spec, []domain.TargetCellSpec{api, web}, []domain.DependencyCellSpec{database})
	ports.config, err = domain.NewTemplates("myapp", []domain.Template{template}, domain.Tmux, domain.Docker, domain.Git, domain.NoNotification)
	if err != nil {
		t.Fatal(err)
	}
	fork := newForkCellUseCase(ports)
	fork.IDs = id.RandomGenerator{}
	note := " API 実装 "
	first, err := fork.Execute(ctx, ForkCellInput{Issue: "118", Template: "feat", Command: "implement 118", Note: &note})
	if err != nil {
		t.Fatal(err)
	}
	second, err := fork.Execute(ctx, ForkCellInput{Issue: "119", Template: "feat"})
	if err != nil {
		t.Fatal(err)
	}
	targets, dependencies := domain.SelectCellGroupMembersService(first.CellGroup.ID, ports.cells.Targets, ports.cells.Dependencies)
	if first.CellGroup.ID == first.ID || first.CellGroup.ID == second.CellGroup.ID || len(targets) != 2 || len(dependencies) != 1 {
		t.Fatalf("invalid grouping: %#v, %#v", first, ports.cells)
	}
	if first.DisplayLabel() != "API 実装" || first.Workspace.Windows[0].Command != "codex implement 118" || first.CellGroup.CreationStatus() != domain.CreationReady {
		t.Fatalf("commands/status/note not preserved: %#v", first)
	}
	ports.cells.Commanders[0].ToggleDone()
	clean := CleanCellUseCase{Cells: ports, SourceFactory: ports, ContainerFactory: ports, WorkspaceFactory: ports}
	if err := clean.Execute(ctx, CleanCellInput{Cell: first.CellGroup.ID}); err != nil {
		t.Fatal(err)
	}
	if len(ports.cells.Commanders) != 1 || ports.cells.Commanders[0].ID != second.ID || len(ports.cells.Targets) != 2 || len(ports.cells.Dependencies) != 1 {
		t.Fatalf("wrong group deleted: %#v", ports.cells)
	}
	if len(ports.cleanedSources) != 2 {
		t.Fatalf("cleaned sources: %#v", ports.cleanedSources)
	}
	for _, resource := range ports.cleanedSources {
		if !strings.Contains(resource.WorktreePath, "/118/") {
			t.Fatalf("unrelated source cleaned: %#v", resource)
		}
	}
}
