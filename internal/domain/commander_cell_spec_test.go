package domain

import "testing"

func TestNewCommanderCellSpecValidatesTargetCardinalityAndReferences(t *testing.T) {
	source, err := NewSourceTemplate(".", "main", "feat/")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewTargetCellSpec("empty", nil, nil, nil); err == nil {
		t.Fatal("target with neither Source nor Container must be rejected")
	}
	if _, err := NewTargetCellSpec("source", &source, nil, nil); err != nil {
		t.Fatalf("source-only target rejected: %v", err)
	}
	mode, _ := NewMode("target")
	container, _ := NewContainerTemplate("web", mode, nil, nil)
	if _, err := NewTargetCellSpec("container", nil, &container, nil); err != nil {
		t.Fatalf("container-only target rejected: %v", err)
	}
	target, _ := NewTargetCellSpec("api", &source, &container, []string{"db"})
	if _, err := NewCommanderCellSpec("work", NewWorkspaceTemplate(nil), []TargetCellSpec{target}, nil); err == nil {
		t.Fatal("target reference to an unrelated dependency must be rejected")
	}
	dependencyContainer, _ := NewContainerTemplate("db", Dependency, nil, nil)
	dependency, _ := NewDependencyCellSpec("db", dependencyContainer)
	if _, err := NewCommanderCellSpec("work", NewWorkspaceTemplate(nil), []TargetCellSpec{target}, []DependencyCellSpec{dependency}); err != nil {
		t.Fatalf("valid cell structure rejected: %v", err)
	}
}

func TestRuntimeCellsAreIndependentTypesAndCommanderHoldsReferences(t *testing.T) {
	workspaceWindow, _ := NewWorkspaceWindow("agent", "codex")
	workspaceDriver, _ := NewWorkspaceDriverType("tmux")
	workspace := NewWorkspace(workspaceDriver, []WorkspaceWindow{workspaceWindow})
	sourceDriver, _ := NewSourceDriverType("git")
	commander, err := NewCommanderCell("commander-id", "113", "sample", "feat", workspace, []string{"target-id"}, []string{"dependency-id"}, sourceDriver, None, NoNotification)
	if err != nil {
		t.Fatal(err)
	}
	if commander.Targets[0] != "target-id" || commander.Dependencies[0] != "dependency-id" || commander.Workspace.Windows[0].Command != "codex" {
		t.Fatalf("commander = %#v", commander)
	}
	if commander.Status != Ready {
		t.Fatalf("initial status = %q, want ready", commander.Status)
	}
	if err := commander.SetStatus(Pending); err != nil || commander.Status != Pending {
		t.Fatalf("set CommanderCell status: %v, status %q", err, commander.Status)
	}
	source, _ := NewSource(".", "main", "feat/api")
	if _, err := NewTargetCell("target-id", "commander-id", "api", &source, nil, []string{"dependency-id"}); err != nil {
		t.Fatalf("source-only runtime TargetCell rejected: %v", err)
	}
	container, _ := NewContainer(nil, "db", Dependency)
	if _, err := NewDependencyCell("dependency-id", "commander-id", "db", container); err != nil {
		t.Fatalf("runtime DependencyCell rejected: %v", err)
	}
}
