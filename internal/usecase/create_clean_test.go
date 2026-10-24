package usecase

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/adapter/id"
	"github.com/hgsg11/paracell/internal/domain"
)

func TestForkCellは新しいTemplateからCellを作る(t *testing.T) {
	ports := newFakePorts()
	usecase := newForkCellUseCase(ports)
	cell, err := usecase.Execute(context.Background(), ForkCellInput{Issue: "42", Template: "feat"})
	if err != nil {
		t.Fatal(err)
	}
	group := ports.groupOf(t, cell)
	if group.Template != "feat" || len(ports.cells.Targets) != 1 || group.Creation.Status != domain.CreationReady {
		t.Fatalf("cell = %#v, group = %#v", cell, group)
	}
	if !reflect.DeepEqual(cell, ports.cells.Commanders[0]) || cell.Version == 0 {
		t.Fatalf("returned cell %#v is not the stored %#v", cell, ports.cells.Commanders[0])
	}
	if got := ports.createdSources[0].Branch; got != "feat/42" {
		t.Fatalf("branch = %q, want feat/42", got)
	}
	if got, want := group.ResourceDrivers(cell.Workspace.Driver), domain.NewCellDrivers(domain.Git, domain.None, domain.Tmux, domain.NoNotification); got != want {
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

func TestForkCellはPrefixをTemplateから独立してSourceへ適用する(t *testing.T) {
	ports := newFakePorts()
	ports.config, _ = domain.NewTemplates("myapp", ports.config.Templates, domain.Tmux, domain.None, domain.Git, domain.NoNotification, map[string]string{"feat": "feature/", "topic": "work-"})
	_, err := newForkCellUseCase(ports).Execute(context.Background(), ForkCellInput{Issue: "42", Template: "feat", Prefix: "topic"})
	if err != nil {
		t.Fatal(err)
	}
	if got := ports.createdSources[0].Branch; got != "work-42" {
		t.Fatalf("branch = %q, want work-42", got)
	}
}

func TestForkCellは未定義Prefixを拒否する(t *testing.T) {
	ports := newFakePorts()
	_, err := newForkCellUseCase(ports).Execute(context.Background(), ForkCellInput{Issue: "42", Template: "feat", Prefix: "unknown"})
	if err == nil || !strings.Contains(err.Error(), `prefix "unknown" not found`) {
		t.Fatalf("error = %v", err)
	}
	if len(ports.cells.Commanders) != 0 || len(ports.cells.Groups) != 0 {
		t.Fatalf("created cells = %#v", ports.cells)
	}
}

func TestForkCellはIssueかCell名が重複するCellを作らない(t *testing.T) {
	for _, input := range []struct{ existing, issue, message string }{
		{existing: "42", issue: "42", message: `issue "42" already exists`},
		{existing: "feat/42", issue: "feat-42", message: `name "feat-42" already exists`},
	} {
		t.Run(input.issue, func(t *testing.T) {
			ports := newFakePorts()
			ports.cells = newUsecaseCellSet(t, [3]string{"cell-0", input.existing, "feat"})
			_, err := newForkCellUseCase(ports).Execute(context.Background(), ForkCellInput{Issue: input.issue, Template: "feat"})
			if err == nil || !strings.Contains(err.Error(), input.message) {
				t.Fatalf("error = %v", err)
			}
			if len(ports.cells.Groups) != 1 || len(ports.cells.Commanders) != 1 || len(ports.calls) != 0 {
				t.Fatalf("cells = %#v, calls = %#v", ports.cells, ports.calls)
			}
		})
	}
}

func TestForkCellはSource作成失敗時も作成対象をCellに保持する(t *testing.T) {
	ports := newFakePorts()
	ports.createSourceErr = errors.New("create source")

	_, err := newForkCellUseCase(ports).Execute(context.Background(), ForkCellInput{Issue: "42", Template: "feat"})
	if !errors.Is(err, ports.createSourceErr) {
		t.Fatalf("error = %v", err)
	}
	if len(ports.cells.Commanders) != 1 || len(ports.cells.Groups) != 1 {
		t.Fatalf("cells = %#v", ports.cells)
	}
	group := ports.cells.Groups[0]
	if group.Creation.Status != domain.CreationFailed || group.Creation.FailedStage != domain.CreationStageSource || group.Creation.LastError != "create source" {
		t.Fatalf("group = %#v", group)
	}
	for _, resource := range domain.BuildSourceResourcesService(group, ports.cells.Targets) {
		if err := ports.CleanSource(context.Background(), resource); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := ports.cleanedSources, []domain.SourceResource{domain.NewSourceResource(".", ".paracell/cells/42/repository/source", "main", "feat/42")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("cleaned sources = %#v, want %#v", got, want)
	}
}

func TestForkCellは保存時の版の競合を作成失敗として伝える(t *testing.T) {
	ports := newFakePorts()
	ports.conflictOnSave = 2
	_, err := newForkCellUseCase(ports).Execute(context.Background(), ForkCellInput{Issue: "42", Template: "feat"})
	if !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(err.Error(), "save source stage") {
		t.Fatalf("error = %v", err)
	}
}

type fakePorts struct {
	config               domain.Templates
	configErr            error
	cells                CellSet
	calls                []string
	saves                int
	conflictOnSave       int
	updateStatusLabelErr error
	createSourceErr      error
	cleanedSources       []domain.SourceResource
	createdSources       []domain.SourceResource
	containerResources   domain.ContainerResources
	workspaceResource    domain.WorkspaceResource
}

func newFakePorts() *fakePorts {
	source, _ := domain.NewSourceTemplate(".", "main")
	target, _ := domain.NewTargetCellSpec("repository", &source, nil)
	commander, _ := domain.NewCommanderCellSpec("workspace", domain.NewWorkspaceTemplate(nil))
	template, _ := domain.NewUnresolvedTemplate("feat", &commander, []domain.TargetCellSpec{target}, nil)
	workspaceDriver, _ := domain.NewWorkspaceDriverType("tmux")
	sourceDriver, _ := domain.NewSourceDriverType("git")
	notificationDriver, _ := domain.NewNotificationDriverType("")
	templates, _ := domain.NewTemplates("myapp", []domain.Template{template}, workspaceDriver, domain.NewContainerDriverType(""), sourceDriver, notificationDriver, nil)
	return &fakePorts{config: templates}
}

func newForkCellUseCase(ports *fakePorts) ForkCellUseCase {
	return ForkCellUseCase{
		Config: ports, Cells: ports, SourceFactory: ports,
		ContainerFactory: ports, WorkspaceFactory: ports, IDs: &sequenceIDGenerator{},
	}
}

func (f *fakePorts) groupOf(t *testing.T, commander domain.CommanderCell) domain.CellGroup {
	t.Helper()
	group, ok := f.cells.group(commander.CellGroupID)
	if !ok {
		t.Fatalf("CellGroup %q not found in %#v", commander.CellGroupID, f.cells.Groups)
	}
	return group
}

func (f *fakePorts) Load(context.Context) (domain.Templates, error) {
	return f.config, f.configErr
}

func (f *fakePorts) LoadCells(context.Context) (CellSet, error) {
	return copyCellSet(f.cells), nil
}

// UpdateCells versions rows like the SQLite adapter: a new CommanderCell must
// have no version and is stored as version 1; a changed row must carry the
// stored version and is stored with it advanced by one.
func (f *fakePorts) UpdateCells(_ context.Context, update func(CellSet) (CellSet, error)) error {
	f.saves++
	next, err := update(copyCellSet(f.cells))
	if err != nil {
		return err
	}
	next = copyCellSet(next)
	if f.saves == f.conflictOnSave {
		return fmt.Errorf("%w: injected", domain.ErrVersionConflict)
	}
	for i, commander := range next.Commanders {
		group, _ := next.group(commander.CellGroupID)
		previous, existed := commanderByID(f.cells.Commanders, commander.ID)
		if !existed {
			if commander.Version != 0 {
				return fmt.Errorf("new CommanderCell %q must not have a version", commander.ID)
			}
			next.Commanders[i].Version = 1
			continue
		}
		previousGroup, _ := f.cells.group(previous.CellGroupID)
		if reflect.DeepEqual(previous, commander) && reflect.DeepEqual(previousGroup, group) {
			continue
		}
		if commander.Version != previous.Version {
			return fmt.Errorf("%w: CommanderCell %q", domain.ErrVersionConflict, commander.ID)
		}
		next.Commanders[i].Version++
	}
	f.cells = next
	return nil
}

func commanderByID(commanders []domain.CommanderCell, id string) (domain.CommanderCell, bool) {
	for _, commander := range commanders {
		if commander.ID == id {
			return commander, true
		}
	}
	return domain.CommanderCell{}, false
}

func copyCellSet(set CellSet) CellSet {
	copied := NewCellSet(set.Groups, set.Commanders, set.Targets, set.Dependencies)
	for i := range copied.Commanders {
		copied.Commanders[i].Workspace = domain.NewWorkspace(copied.Commanders[i].Workspace.Driver, copied.Commanders[i].Workspace.Windows)
	}
	return copied
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

func (f *fakePorts) CreateSource(_ context.Context, resource domain.SourceResource) error {
	f.calls = append(f.calls, "source:create")
	f.createdSources = append(f.createdSources, resource)
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
func (f *fakePorts) EnterCommand(_ context.Context, resource domain.WorkspaceResource) (*exec.Cmd, error) {
	f.calls = append(f.calls, "workspace:enter-command:"+resource.CellName)
	f.workspaceResource = resource
	return exec.Command("tmux", "switch-client", "-t", resource.Name), nil
}
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

type sequenceIDGenerator struct{ next int }

func (g *sequenceIDGenerator) NewID() string {
	g.next++
	return fmt.Sprintf("cell-%d", g.next)
}

// newUsecaseTestCell returns a persisted CellGroup "group-<id>" and its
// CommanderCell "<id>" at version 1.
func newUsecaseTestCell(t *testing.T, id string, issue string, templateName string) (domain.CellGroup, domain.CommanderCell) {
	t.Helper()
	group, err := domain.NewCellGroup("group-"+id, issue, "myapp", templateName, domain.Git, domain.None, domain.NoNotification)
	if err != nil {
		t.Fatal(err)
	}
	cell, err := domain.CommanderCellFactory{}.Restore(id, group.ID, "tmux", nil, "ready", false, 1)
	if err != nil {
		t.Fatal(err)
	}
	return group, cell
}

// newUsecaseCellSet builds a persisted CellSet from {id, issue, template} triples.
func newUsecaseCellSet(t *testing.T, cells ...[3]string) CellSet {
	t.Helper()
	set := NewCellSet(nil, nil, nil, nil)
	for _, cell := range cells {
		group, commander := newUsecaseTestCell(t, cell[0], cell[1], cell[2])
		set.Groups = append(set.Groups, group)
		set.Commanders = append(set.Commanders, commander)
	}
	return set
}

func TestCellGroupsPreserveTemplateLinksAndCleanOnlySelectedGroup(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	source, _ := domain.NewSourceTemplate(".", "main")
	app, _ := domain.NewContainerTemplate("app", domain.Target, nil, nil)
	api, _ := domain.NewTargetCellSpec("api", &source, []domain.ContainerTemplate{app})
	web, _ := domain.NewTargetCellSpec("web", &source, nil)
	database, _ := domain.NewDependencyCellSpec("database")
	window, _ := domain.NewWindow("agent", "codex {{.Command}}")
	spec, err := domain.NewCommanderCellSpec("workspace", domain.NewWorkspaceTemplate([]domain.Window{window}))
	if err != nil {
		t.Fatal(err)
	}
	template, _ := domain.NewUnresolvedTemplate("feat", &spec, []domain.TargetCellSpec{api, web}, []domain.DependencyCellSpec{database})
	ports.config, err = domain.NewTemplates("myapp", []domain.Template{template}, domain.Tmux, domain.Docker, domain.Git, domain.NoNotification, nil)
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
	firstGroup := ports.groupOf(t, first)
	targets, dependencies := domain.SelectCellGroupMembersService(first.CellGroupID, ports.cells.Targets, ports.cells.Dependencies)
	if first.CellGroupID == first.ID || first.CellGroupID == second.CellGroupID || len(targets) != 2 || len(dependencies) != 1 {
		t.Fatalf("invalid grouping: %#v, %#v", first, ports.cells)
	}
	if firstGroup.WorkspaceResource(first.Workspace).DisplayLabel != "API 実装" || first.Workspace.Windows[0].Command != "codex implement 118" || firstGroup.Creation.Status != domain.CreationReady {
		t.Fatalf("commands/status/note not preserved: %#v / %#v", first, firstGroup)
	}
	if err := ports.cells.Commanders[0].MarkDone(); err != nil {
		t.Fatal(err)
	}
	clean := CleanCellUseCase{Cells: ports, SourceFactory: ports, ContainerFactory: ports, WorkspaceFactory: ports}
	if err := clean.Execute(ctx, CleanCellInput{Cell: first.CellGroupID}); err != nil {
		t.Fatal(err)
	}
	if len(ports.cells.Groups) != 1 || len(ports.cells.Commanders) != 1 || ports.cells.Commanders[0].ID != second.ID || len(ports.cells.Targets) != 2 || len(ports.cells.Dependencies) != 1 {
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

func TestCleanCellは未完了のCellを消さない(t *testing.T) {
	ports := newFakePorts()
	ports.cells = newUsecaseCellSet(t, [3]string{"cell-1", "123", "feat"})
	clean := CleanCellUseCase{Cells: ports, SourceFactory: ports, ContainerFactory: ports, WorkspaceFactory: ports}
	if err := clean.Execute(context.Background(), CleanCellInput{Cell: "123"}); err == nil {
		t.Fatal("an undone cell was cleaned")
	}
	if len(ports.cells.Groups) != 1 || len(ports.calls) != 0 {
		t.Fatalf("cells = %#v, calls = %#v", ports.cells, ports.calls)
	}
}

func TestCleanCellは見つからないCellを伝える(t *testing.T) {
	ports := newFakePorts()
	ports.cells = newUsecaseCellSet(t, [3]string{"cell-1", "123", "feat"})
	ports.cells.Commanders[0].Done = true
	clean := CleanCellUseCase{Cells: ports, SourceFactory: ports, ContainerFactory: ports, WorkspaceFactory: ports}
	// The CommanderCell ID is not a Cell identifier.
	for _, identifier := range []string{"missing", "cell-1"} {
		if err := clean.Execute(context.Background(), CleanCellInput{Cell: identifier}); err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("Clean(%q) error = %v", identifier, err)
		}
	}
	if len(ports.cells.Groups) != 1 || len(ports.calls) != 0 {
		t.Fatalf("cells = %#v, calls = %#v", ports.cells, ports.calls)
	}
}
