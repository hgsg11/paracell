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
	display := ports.cells[0].Display()
	if display.CreationStatus != domain.CreationFailed || display.FailedStage != domain.CreationStageSource {
		t.Fatalf("display = %#v", display)
	}
	if err := domain.CleanSources(context.Background(), ports.cells[0], ports); err != nil {
		t.Fatal(err)
	}
	if got, want := ports.cleanedSources, []domain.SourceResource{domain.NewSourceResource(".", ".paracell/cells/42/source", "main", "feat/42")}; !reflect.DeepEqual(got, want) {
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
	cleanedSources       []domain.SourceResource
	containerResources   domain.ContainerResources
	sessionResource      domain.SessionResource
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
		Config: ports, State: ports, CellFactory: ports, SourceFactory: ports,
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

func (f *fakePorts) Source(driver domain.SourceDriverType) (domain.SourcePort, error) {
	f.calls = append(f.calls, "factory:source:"+string(driver))
	return f, nil
}

func (f *fakePorts) Container(driver domain.ContainerDriverType) (domain.ContainerPort, error) {
	f.calls = append(f.calls, "factory:container:"+string(driver))
	return f, nil
}

func (f *fakePorts) Session(driver domain.SessionDriverType) (domain.SessionPort, error) {
	f.calls = append(f.calls, "factory:session:"+string(driver))
	return f, nil
}

func (f *fakePorts) NewCell(id string, issue string, project string, templateName string, sources domain.Sources, containers domain.Containers, session domain.Session, notificationDriver domain.NotificationDriverType) (domain.Cell, error) {
	return domain.NewCell(id, issue, project, templateName, sources, containers, session, notificationDriver)
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
func (f *fakePorts) CreateSession(_ context.Context, resource domain.SessionResource) error {
	f.calls = append(f.calls, "session:create")
	f.sessionResource = resource
	return nil
}
func (f *fakePorts) CleanSession(context.Context, domain.SessionResource) error {
	f.calls = append(f.calls, "session:clean")
	return nil
}
func (f *fakePorts) PrepareSession(context.Context, domain.SessionResource) error { return nil }
func (f *fakePorts) UpdateStatusLabel(_ context.Context, resource domain.SessionResource) error {
	f.calls = append(f.calls, "session:label:"+resource.DisplayLabel)
	return f.updateStatusLabelErr
}
func (f *fakePorts) EnterSession(_ context.Context, resource domain.SessionResource) error {
	f.calls = append(f.calls, "session:enter:"+resource.CellName)
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
