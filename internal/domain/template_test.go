package domain

import (
	"strings"
	"testing"
)

func TestResolveTemplateはCommanderCell仕様とRuntime変数を解決する(t *testing.T) {
	source, _ := NewSourceTemplate("services/api", "origin/main", "feat/")
	mode, _ := NewMode("target")
	environment, _ := NewEnvironment("CELL", "{{.Project}}-{{.Name}}")
	container, _ := NewContainerTemplate("api", mode, []Environment{environment}, nil)
	target, _ := NewTargetCellSpec("api", &source, &container, []string{"db"})
	dependencyContainer, _ := NewContainerTemplate("postgres", Dependency, nil, nil)
	dependency, _ := NewDependencyCellSpec("db", dependencyContainer)
	commander, err := NewCommanderCellSpec("workspace", NewWorkspaceTemplate([]Window{{Name: "agent", Command: "codex {{.Command}}"}}), []TargetCellSpec{target}, []DependencyCellSpec{dependency})
	if err != nil {
		t.Fatal(err)
	}
	base, _ := NewUnresolvedTemplate("base", "", true, &commander)
	child, _ := NewUnresolvedTemplate("feat", "base", false, nil)
	workspaceDriver, _ := NewWorkspaceDriverType("tmux")
	sourceDriver, _ := NewSourceDriverType("git")
	config, _ := NewTemplates("sample", []Template{base, child}, workspaceDriver, Docker, sourceDriver, NoNotification)

	resolved, err := config.Resolve("feat", NewTemplateVars("42", "42", "sample", "make test"))
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Commander == nil || resolved.Commander.Name != "workspace" || resolved.Commander.Workspace.Windows[0].Command != "codex make test" {
		t.Fatalf("resolved CommanderCell = %#v", resolved.Commander)
	}
	api := resolved.Commander.Targets[0]
	if api.Source == nil || api.Source.Path != "services/api" || api.Container == nil || api.Container.Environments[0].Value != "sample-42" {
		t.Fatalf("resolved TargetCellSpec = %#v", api)
	}
	if api.Dependencies[0] != "db" || resolved.Commander.Dependencies[0].Container.Name != "postgres" {
		t.Fatalf("Cell references = %#v / %#v", api.Dependencies, resolved.Commander.Dependencies)
	}
}

func TestResolveTemplateはInheritanceCycleとAbstractTemplateを拒否する(t *testing.T) {
	workspaceDriver, _ := NewWorkspaceDriverType("tmux")
	sourceDriver, _ := NewSourceDriverType("git")
	a, _ := NewUnresolvedTemplate("a", "b", false, nil)
	b, _ := NewUnresolvedTemplate("b", "a", false, nil)
	config, _ := NewTemplates("sample", []Template{a, b}, workspaceDriver, None, sourceDriver, NoNotification)
	if _, err := config.Resolve("a", NewTemplateVars("", "", "", "")); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("cycle error = %v", err)
	}
	window, _ := NewWindow("agent", "")
	commander, _ := NewCommanderCellSpec("workspace", NewWorkspaceTemplate([]Window{window}), nil, nil)
	base, _ := NewUnresolvedTemplate("base", "", true, &commander)
	config, _ = NewTemplates("sample", []Template{base}, workspaceDriver, None, sourceDriver, NoNotification)
	if _, err := config.Resolve("base", NewTemplateVars("", "", "", "")); err == nil {
		t.Fatal("abstract template must not be selectable")
	}
	if _, err := config.Resolve("missing", NewTemplateVars("", "", "", "")); err == nil {
		t.Fatal("missing template must be rejected")
	}
}
