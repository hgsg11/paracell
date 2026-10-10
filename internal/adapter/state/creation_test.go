package state

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
	"github.com/hgsg11/paracell/internal/usecase"
)

// creationPorts substitutes external resources while retaining real SQLite persistence.
type creationPorts struct {
	usecase.WorkspacePort
	config      domain.Templates
	failedStage domain.CreationStage
	calls       []string
	nextID      int
}

func newCreationPorts(t *testing.T, failedStage domain.CreationStage) *creationPorts {
	t.Helper()
	source, _ := domain.NewSourceTemplate(".", "main")
	repository, _ := domain.NewTargetCellSpec("repository", &source, nil)
	web, _ := domain.NewContainerTemplate("web", domain.Target, nil, nil)
	target, _ := domain.NewTargetCellSpec("web", nil, []domain.ContainerTemplate{web})
	dependency, _ := domain.NewDependencyCellSpec("database")
	commander, _ := domain.NewCommanderCellSpec("workspace", domain.NewWorkspaceTemplate(nil))
	template, _ := domain.NewUnresolvedTemplate("verify", "", false, &commander, []domain.TargetCellSpec{repository, target}, []domain.DependencyCellSpec{dependency})
	config, err := domain.NewTemplates("verify", []domain.Template{template}, domain.Tmux, domain.Docker, domain.Git, domain.NoNotification, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &creationPorts{config: config, failedStage: failedStage}
}

func (p *creationPorts) NewID() string { p.nextID++; return fmt.Sprintf("cell-%d", p.nextID) }

func (p *creationPorts) Load(context.Context) (domain.Templates, error)             { return p.config, nil }
func (p *creationPorts) Source(domain.SourceDriverType) (usecase.SourcePort, error) { return p, nil }
func (p *creationPorts) Container(domain.ContainerDriverType) (usecase.ContainerPort, error) {
	return p, nil
}
func (p *creationPorts) Workspace(domain.WorkspaceDriverType) (usecase.WorkspacePort, error) {
	return p, nil
}
func (p *creationPorts) CreateSource(context.Context, domain.SourceResource) error {
	p.calls = append(p.calls, "source")
	if p.failedStage == domain.CreationStageSource {
		return errors.New("source failed")
	}
	return nil
}
func (p *creationPorts) CleanSource(context.Context, domain.SourceResource) error {
	p.calls = append(p.calls, "clean source")
	return nil
}
func (p *creationPorts) CreateContainers(context.Context, domain.ContainerResources) (map[string][]string, error) {
	p.calls = append(p.calls, "containers")
	if p.failedStage == domain.CreationStageContainers {
		return nil, errors.New("containers failed")
	}
	return map[string][]string{"web": {"cell-network"}, "database": {"cell-network"}}, nil
}
func (p *creationPorts) CleanContainers(context.Context, domain.ContainerResources) error {
	p.calls = append(p.calls, "clean containers")
	return nil
}
func (p *creationPorts) CreateWorkspace(context.Context, domain.WorkspaceResource) error {
	p.calls = append(p.calls, "workspace")
	if p.failedStage == domain.CreationStageWorkspace {
		return errors.New("workspace failed")
	}
	return nil
}
func (p *creationPorts) CleanWorkspace(context.Context, domain.WorkspaceResource) error {
	p.calls = append(p.calls, "clean workspace")
	return nil
}

func TestCellCreationPersistsAllStages(t *testing.T) {
	for _, failedStage := range []domain.CreationStage{"", domain.CreationStageSource, domain.CreationStageContainers, domain.CreationStageWorkspace} {
		t.Run(string(failedStage), func(t *testing.T) {
			ctx := context.Background()
			adapter := NewSQLiteCellAdapter(filepath.Join(t.TempDir(), "state.db"))
			ports := newCreationPorts(t, failedStage)
			fork := usecase.ForkCellUseCase{Config: ports, Cells: adapter, SourceFactory: ports, ContainerFactory: ports, WorkspaceFactory: ports, IDs: ports}
			commander, err := fork.Execute(ctx, usecase.ForkCellInput{Issue: "120", Template: "verify"})
			if failedStage == "" && err != nil {
				t.Fatal(err)
			}
			if failedStage != "" && (err == nil || err.Error() != string(failedStage)+" failed") {
				t.Fatalf("error = %v", err)
			}
			stored, err := adapter.LoadCells(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(stored.Commanders) != 1 || len(stored.Targets) != 2 || len(stored.Dependencies) != 1 {
				t.Fatalf("incomplete CellGroup: %#v", stored)
			}
			saved := stored.Commanders[0]
			if failedStage == "" {
				if saved.CreationStatus() != domain.CreationReady || !reflect.DeepEqual(commander.Stored(), saved.Stored()) {
					t.Fatalf("returned/persisted commander mismatch: %#v / %#v", commander, saved)
				}
				if !reflect.DeepEqual(ports.calls, []string{"source", "containers", "workspace"}) {
					t.Fatalf("stages = %v", ports.calls)
				}
				if !reflect.DeepEqual(stored.Targets[1].Containers[0].Network, []string{"cell-network"}) || !reflect.DeepEqual(stored.Dependencies[0].Container.Network, []string{"cell-network"}) {
					t.Fatal("container networks not persisted")
				}
			} else {
				stage, message := saved.CreationFailure()
				if saved.CreationStatus() != domain.CreationFailed || stage != failedStage || message != string(failedStage)+" failed" {
					t.Fatalf("failure not persisted: %#v", saved)
				}
				if failedStage == domain.CreationStageWorkspace && !reflect.DeepEqual(ports.calls, []string{"source", "containers", "workspace", "clean containers"}) {
					t.Fatalf("rollback calls = %v", ports.calls)
				}
			}
			// Both completed and failed groups remain cleanable through the normal use case.
			if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) { return set, set.Commanders[0].MarkDone() }); err != nil {
				t.Fatal(err)
			}
			clean := usecase.CleanCellUseCase{Cells: adapter, SourceFactory: ports, ContainerFactory: ports, WorkspaceFactory: ports}
			if err := clean.Execute(ctx, usecase.CleanCellInput{Cell: "120"}); err != nil {
				t.Fatal(err)
			}
			stored, err = adapter.LoadCells(ctx)
			if err != nil || len(stored.Commanders)+len(stored.Targets)+len(stored.Dependencies) != 0 {
				t.Fatalf("cleanup: %#v, %v", stored, err)
			}
		})
	}
}
