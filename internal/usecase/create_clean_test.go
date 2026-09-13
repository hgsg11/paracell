package usecase

import (
	"context"
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
	if cell.Template != "feat" || len(cell.Sources) != 1 || cell.CreationStatus() != domain.CreationReady {
		t.Fatalf("cell = %#v", cell)
	}
}

type fakePorts struct {
	config               domain.Templates
	configErr            error
	cells                []domain.Cell
	calls                []string
	updateStatusLabelErr error
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

func (f *fakePorts) Load(context.Context, *domain.TemplateVars) (domain.Templates, error) {
	return f.config, f.configErr
}

func (f *fakePorts) LoadCells(context.Context) ([]domain.Cell, error) {
	return append([]domain.Cell(nil), f.cells...), nil
}

func (f *fakePorts) UpdateCells(_ context.Context, update func([]domain.Cell) ([]domain.Cell, error)) error {
	cells, err := update(append([]domain.Cell(nil), f.cells...))
	if err != nil {
		return err
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

func (f *fakePorts) Session(driver domain.SessionDriverType) (SessionPort, error) {
	f.calls = append(f.calls, "factory:session:"+string(driver))
	return f, nil
}

func (f *fakePorts) NewCell(id string, issue string, templateName string, sources []domain.SourceTemplate, containers []domain.ContainerTemplate, session domain.SessionTemplate, project string) (domain.Cell, error) {
	cellSources := make([]domain.Source, 0, len(sources))
	for _, source := range sources {
		cellSources = append(cellSources, domain.Source{TemplatePath: source.Path, Path: "source", Base: source.Base, Branch: source.Prefix + issue})
	}
	return domain.Cell{ID: id, Issue: issue, Name: issue, Template: templateName, Sources: cellSources, Session: domain.Session{Name: project + "-" + issue}}, nil
}

func (f *fakePorts) CreateSource(context.Context, domain.Cell) (SourceCreation, error) {
	f.calls = append(f.calls, "source:create")
	return SourceCreation{}, nil
}
func (f *fakePorts) ResumeSource(context.Context, domain.Cell) error { return nil }
func (f *fakePorts) CleanSource(context.Context, domain.Cell) error {
	f.calls = append(f.calls, "source:clean")
	return nil
}
func (f *fakePorts) CreateContainers(_ context.Context, _ domain.Cell, _ []domain.ContainerTemplate) error {
	f.calls = append(f.calls, "containers:create")
	return nil
}
func (f *fakePorts) CleanContainers(context.Context, domain.Cell) error {
	f.calls = append(f.calls, "containers:clean")
	return nil
}
func (f *fakePorts) CreateSession(context.Context, domain.Cell) error {
	f.calls = append(f.calls, "session:create")
	return nil
}
func (f *fakePorts) CleanSession(context.Context, domain.Cell) error {
	f.calls = append(f.calls, "session:clean")
	return nil
}
func (f *fakePorts) PrepareSession(context.Context, domain.Cell) error { return nil }
func (f *fakePorts) UpdateStatusLabel(_ context.Context, cell domain.Cell) error {
	f.calls = append(f.calls, "session:label:"+cell.DisplayLabel())
	return f.updateStatusLabelErr
}
func (f *fakePorts) EnterSession(_ context.Context, cell domain.Cell) error {
	f.calls = append(f.calls, "session:enter:"+cell.Name)
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
