package usecase

import (
	"context"
	"errors"
	"reflect"
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
	if cell.Template != "feat" || len(cell.Sources.Items) != 1 || cell.CreationStatus() != domain.CreationReady {
		t.Fatalf("cell = %#v", cell)
	}
	if got, want := cell.ResourceDrivers(), domain.NewCellDrivers(domain.Git, domain.None, domain.Tmux, domain.NoNotification); got != want {
		t.Fatalf("drivers = %#v, want %#v", got, want)
	}
	wantCalls := []string{
		"factory:source:git",
		"factory:container:none",
		"factory:session:tmux",
		"source:create",
		"containers:create",
		"session:create",
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
	if len(ports.cells) != 1 {
		t.Fatalf("cells = %d", len(ports.cells))
	}
	failedStage, _ := ports.cells[0].CreationFailure()
	if ports.cells[0].CreationStatus() != domain.CreationFailed || failedStage != domain.CreationStageSource {
		t.Fatalf("cell = %#v", ports.cells[0])
	}
	repositories, worktrees := ports.cells[0].SourceCleanupTargets()
	for i, repository := range repositories {
		if err := ports.CleanSource(context.Background(), repository, worktrees[i]); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := ports.cleanedSources, map[string]string{".": ".paracell/cells/42/source"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("cleaned sources = %#v, want %#v", got, want)
	}
}

type fakePorts struct {
	config               domain.Templates
	configErr            error
	cells                []domain.Cell
	calls                []string
	updateStatusLabelErr error
	createSourceErr      error
	cleanedSources       map[string]string
	onCreateSource       func(domain.SourceTemplate, string, string)
	onCreateContainers   func([]domain.ContainerTemplate, string, string, string, string)
	onCreateSession      func(domain.SessionTemplate, string, string, string, string, string)
	createSessionErr     error
	saveCalls            int
	failSaveAt           int
	saveErr              error
	cleanedNetwork       string
	cleanedContainers    []string
	cleanedDependencies  []string
	cleanedSession       string
	cleanContainersErr   error
}

func newFakePorts() *fakePorts {
	source, _ := domain.NewSourceTemplate(".", "main", "feat/")
	template, _ := domain.NewTemplate("feat", []domain.SourceTemplate{source}, nil, domain.NewSessionTemplate(nil))
	sessionDriver, _ := domain.NewSessionDriverType("tmux")
	sourceDriver, _ := domain.NewSourceDriverType("git")
	notificationDriver, _ := domain.NewNotificationDriverType("")
	templates, _ := domain.NewTemplates("myapp", []domain.Template{template}, sessionDriver, domain.NewContainerDriverType(""), sourceDriver, notificationDriver)
	return &fakePorts{config: templates}
}

func newForkCellUseCase(ports *fakePorts) ForkCellUseCase {
	return ForkCellUseCase{
		Config: ports, Cells: ports, SourceFactory: ports,
		ContainerFactory: ports, SessionFactory: ports, IDs: fixedIDGenerator{id: "cell-1"},
	}
}

func (f *fakePorts) Load(context.Context) (domain.Templates, error) {
	return f.config, f.configErr
}

func (f *fakePorts) LoadCells(context.Context) ([]domain.Cell, error) {
	return append([]domain.Cell(nil), f.cells...), nil
}

func (f *fakePorts) UpdateCells(_ context.Context, update func([]domain.Cell) ([]domain.Cell, error)) error {
	f.saveCalls++
	if f.saveCalls == f.failSaveAt {
		return f.saveErr
	}
	before := append([]domain.Cell(nil), f.cells...)
	cells, err := update(append([]domain.Cell(nil), f.cells...))
	if err != nil {
		return err
	}
	for index := range cells {
		for _, previous := range before {
			if previous.ID == cells[index].ID && !reflect.DeepEqual(previous, cells[index]) {
				if err := cells[index].AdvanceVersion(); err != nil {
					return err
				}
			}
		}
	}
	f.cells = cells
	return nil
}

func (f *fakePorts) DeleteCell(_ context.Context, target domain.Cell) error {
	for index, cell := range f.cells {
		if cell.ID == target.ID {
			if cell.Version != target.Version {
				return domain.ErrVersionConflict
			}
			f.cells = append(f.cells[:index], f.cells[index+1:]...)
			return nil
		}
	}
	return domain.ErrNotFound
}

func (f *fakePorts) Source(driver domain.SourceDriverType) (SourcePort, error) {
	f.calls = append(f.calls, "factory:source:"+string(driver))
	return f, nil
}

func (f *fakePorts) Container(driver domain.ContainerDriverType) (ContainerPort, error) {
	f.calls = append(f.calls, "factory:container:"+string(driver))
	return f, nil
}

func (f *fakePorts) Session(driver domain.SessionDriverType) (SessionPort, error) {
	f.calls = append(f.calls, "factory:session:"+string(driver))
	return f, nil
}

func (f *fakePorts) CreateSource(_ context.Context, template domain.SourceTemplate, worktree string, branch string) error {
	f.calls = append(f.calls, "source:create")
	if f.onCreateSource != nil {
		f.onCreateSource(template, worktree, branch)
	}
	return f.createSourceErr
}
func (f *fakePorts) CleanSource(_ context.Context, repository string, worktree string) error {
	f.calls = append(f.calls, "source:clean")
	if f.cleanedSources == nil {
		f.cleanedSources = make(map[string]string)
	}
	f.cleanedSources[repository] = worktree
	return nil
}
func (f *fakePorts) CreateContainers(_ context.Context, templates []domain.ContainerTemplate, cellName string, project string, network string, sourcePath string) (map[string][]string, error) {
	f.calls = append(f.calls, "containers:create")
	if f.onCreateContainers != nil {
		f.onCreateContainers(templates, cellName, project, network, sourcePath)
	}
	return map[string][]string{"app": {"original_default"}}, nil
}
func (f *fakePorts) CleanContainers(_ context.Context, network string, containers []string, dependencies []string) error {
	f.calls = append(f.calls, "containers:clean")
	f.cleanedNetwork, f.cleanedContainers, f.cleanedDependencies = network, containers, dependencies
	return f.cleanContainersErr
}
func (f *fakePorts) CreateSession(_ context.Context, template domain.SessionTemplate, name string, cellName string, project string, label string, workingDirectory string) error {
	f.calls = append(f.calls, "session:create")
	if f.onCreateSession != nil {
		f.onCreateSession(template, name, cellName, project, label, workingDirectory)
	}
	return f.createSessionErr
}
func (f *fakePorts) CleanSession(_ context.Context, name string) error {
	f.calls = append(f.calls, "session:clean")
	f.cleanedSession = name
	return nil
}
func (f *fakePorts) PrepareSession(context.Context, string, string, string, string, []string) error {
	return nil
}
func (f *fakePorts) UpdateStatusLabel(_ context.Context, name string, label string) error {
	f.calls = append(f.calls, "session:label:"+label)
	return f.updateStatusLabelErr
}
func (f *fakePorts) EnterSession(_ context.Context, name string, cellName string, project string, label string, windows []string) error {
	f.calls = append(f.calls, "session:enter:"+cellName)
	return nil
}
func (f *fakePorts) EnterRootSession(_ context.Context, projectName string) error {
	f.calls = append(f.calls, "session:enter-root:"+projectName)
	return nil
}
func (f *fakePorts) ExitSession(context.Context) error {
	f.calls = append(f.calls, "session:exit")
	return nil
}

type fixedIDGenerator struct{ id string }

func (g fixedIDGenerator) NewID() string { return g.id }

func newUsecaseTestCell(t *testing.T, id string, issue string, templateName string) domain.Cell {
	t.Helper()
	sourceDriver, _ := domain.NewSourceDriverType("git")
	sessionDriver, _ := domain.NewSessionDriverType("tmux")
	cell, err := domain.NewCell(id, issue, "myapp", templateName, domain.NewSources(sourceDriver, nil), domain.NewContainers(domain.None, nil), domain.NewSession(sessionDriver, nil), domain.NoNotification)
	if err != nil {
		t.Fatal(err)
	}
	return cell
}

func newConfiguredCreationPorts(t *testing.T) *fakePorts {
	t.Helper()
	ports := newFakePorts()
	source, err := domain.NewSourceTemplate("api", "develop", "work/")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := domain.NewEnvironment("ISSUE", "{{.Issue}}")
	if err != nil {
		t.Fatal(err)
	}
	mount, err := domain.NewMount("/app", ".")
	if err != nil {
		t.Fatal(err)
	}
	app, err := domain.NewContainerTemplate("app", domain.Target, []domain.Environment{environment}, []domain.Mount{mount})
	if err != nil {
		t.Fatal(err)
	}
	db, err := domain.NewContainerTemplate("db", domain.Dependency, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	window, err := domain.NewWindow("editor", "echo {{.Issue}}")
	if err != nil {
		t.Fatal(err)
	}
	parent, err := domain.NewTemplate("base", []domain.SourceTemplate{source}, []domain.ContainerTemplate{app, db}, domain.NewSessionTemplate([]domain.Window{window}))
	if err != nil {
		t.Fatal(err)
	}
	child, err := domain.NewUnresolvedTemplate("feat", "base", false, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ports.config, err = domain.NewTemplates("myapp", []domain.Template{parent, child}, domain.Tmux, domain.Docker, domain.Git, domain.NoNotification)
	if err != nil {
		t.Fatal(err)
	}
	return ports
}

func TestForkCellは解決済みTemplateと実行時引数を渡しNetworkを保存する(t *testing.T) {
	ports := newConfiguredCreationPorts(t)
	ports.onCreateSource = func(template domain.SourceTemplate, worktree, branch string) {
		if template.Path != "api" || template.Base != "develop" || worktree != ".paracell/cells/42/source/api" || branch != "work/42" {
			t.Fatalf("source = %#v, %q, %q", template, worktree, branch)
		}
	}
	ports.onCreateContainers = func(templates []domain.ContainerTemplate, cellName, project, network, sourcePath string) {
		if cellName != "42" || project != "myapp" || network != "paracell-myapp-42" || sourcePath != ".paracell/cells/42/source/api" {
			t.Fatalf("container arguments = %q %q %q %q", cellName, project, network, sourcePath)
		}
		if len(templates) != 2 || templates[0].Name != "app" || templates[0].Mode != domain.Target || templates[0].Environments[0].Value != "42" || templates[0].Mounts[0].TargetPath != "/app" || templates[1].Mode != domain.Dependency {
			t.Fatalf("templates = %#v", templates)
		}
	}
	ports.onCreateSession = func(template domain.SessionTemplate, name, cellName, project, label, directory string) {
		if name != "myapp-42" || cellName != "42" || project != "myapp" || label != "作業中" || directory != ".paracell/cells/42/source/api" {
			t.Fatalf("session arguments = %q %q %q %q %q", name, cellName, project, label, directory)
		}
		if len(template.Windows) != 1 || template.Windows[0].Name != "editor" || template.Windows[0].Command != "echo 42" {
			t.Fatalf("session template = %#v", template)
		}
	}
	note := "作業中"
	_, err := newForkCellUseCase(ports).Execute(context.Background(), ForkCellInput{Issue: "42", Template: "feat", Note: &note})
	if err != nil {
		t.Fatal(err)
	}
	if got := ports.cells[0].Containers.Items[0].Network; !reflect.DeepEqual(got, []string{"original_default"}) {
		t.Fatalf("saved networks = %v", got)
	}
}

func TestCleanCellは現在のTemplateなしで保存済み対象を削除する(t *testing.T) {
	ports := newConfiguredCreationPorts(t)
	_, err := newForkCellUseCase(ports).Execute(context.Background(), ForkCellInput{Issue: "42", Template: "feat"})
	if err != nil {
		t.Fatal(err)
	}
	if err := ports.cells[0].MarkDone(); err != nil {
		t.Fatal(err)
	}
	ports.configErr = errors.New("configuration no longer exists")
	ports.cleanContainersErr = domain.ErrNotFound
	err = (CleanCellUseCase{Cells: ports, SourceFactory: ports, ContainerFactory: ports, SessionFactory: ports}).Execute(context.Background(), CleanCellInput{Cell: "42"})
	if err != nil {
		t.Fatal(err)
	}
	if ports.cleanedSession != "myapp-42" || ports.cleanedNetwork != "paracell-myapp-42" || !reflect.DeepEqual(ports.cleanedContainers, []string{"paracell-myapp-42-app"}) || !reflect.DeepEqual(ports.cleanedDependencies, []string{"db"}) || !reflect.DeepEqual(ports.cleanedSources, map[string]string{"api": ".paracell/cells/42/source/api"}) || len(ports.cells) != 0 {
		t.Fatalf("cleanup = %#v", ports)
	}
}

func TestForkCellはSession失敗時にDependencyをRollbackする(t *testing.T) {
	ports := newConfiguredCreationPorts(t)
	ports.createSessionErr = errors.New("session failed")
	ports.cleanContainersErr = errors.New("disconnect failed")
	_, err := newForkCellUseCase(ports).Execute(context.Background(), ForkCellInput{Issue: "42", Template: "feat"})
	if !errors.Is(err, ports.createSessionErr) || !errors.Is(err, ports.cleanContainersErr) {
		t.Fatalf("error = %v", err)
	}
	if ports.cleanedNetwork != "paracell-myapp-42" || !reflect.DeepEqual(ports.cleanedContainers, []string{"paracell-myapp-42-app"}) || !reflect.DeepEqual(ports.cleanedDependencies, []string{"db"}) {
		t.Fatalf("rollback = %#v", ports)
	}
	stage, _ := ports.cells[0].CreationFailure()
	if stage != domain.CreationStageSession || ports.cells[0].CreationStatus() != domain.CreationFailed {
		t.Fatalf("stored = %#v", ports.cells[0])
	}
}

func TestForkCellは保存失敗時に未保存StageをCleanupする(t *testing.T) {
	for _, stage := range []domain.CreationStage{domain.CreationStageContainers, domain.CreationStageSession} {
		t.Run(string(stage), func(t *testing.T) {
			ports := newConfiguredCreationPorts(t)
			ports.failSaveAt = 3
			if stage == domain.CreationStageSession {
				ports.failSaveAt = 4
			}
			ports.saveErr = errors.New("save failed")
			_, err := newForkCellUseCase(ports).Execute(context.Background(), ForkCellInput{Issue: "42", Template: "feat"})
			if !errors.Is(err, ports.saveErr) {
				t.Fatalf("error = %v", err)
			}
			if ports.cleanedNetwork != "paracell-myapp-42" || !reflect.DeepEqual(ports.cleanedContainers, []string{"paracell-myapp-42-app"}) || !reflect.DeepEqual(ports.cleanedDependencies, []string{"db"}) {
				t.Fatalf("container cleanup = %#v", ports)
			}
			if stage == domain.CreationStageSession && ports.cleanedSession != "myapp-42" {
				t.Fatalf("session cleanup = %q", ports.cleanedSession)
			}
			failed, _ := ports.cells[0].CreationFailure()
			if failed != stage || ports.cells[0].CreationStatus() != domain.CreationFailed {
				t.Fatalf("stored = %#v", ports.cells[0])
			}
		})
	}
}
