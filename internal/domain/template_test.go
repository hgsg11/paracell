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
	target, _ := NewTargetCellSpec("api", &source, []ContainerTemplate{container})
	dependency, _ := NewDependencyCellSpec("postgres")
	commander, err := NewCommanderCellSpec("workspace", NewWorkspaceTemplate([]Window{{Name: "agent", Command: "codex {{.Command}}"}}))
	if err != nil {
		t.Fatal(err)
	}
	base, _ := NewUnresolvedTemplate("base", "", true, &commander, []TargetCellSpec{target}, []DependencyCellSpec{dependency})
	child, _ := NewUnresolvedTemplate("feat", "base", false, nil, nil, nil)
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
	api := resolved.Targets[0]
	if api.Source == nil || api.Source.Path != "services/api" || len(api.Containers) != 1 || api.Containers[0].Environments[0].Value != "sample-42" {
		t.Fatalf("resolved TargetCellSpec = %#v", api)
	}
	if resolved.Dependencies[0].Name != "postgres" {
		t.Fatalf("DependencyCellSpec = %#v", resolved.Dependencies)
	}
}

func TestResolveTemplateはInheritanceCycleとAbstractTemplateを拒否する(t *testing.T) {
	workspaceDriver, _ := NewWorkspaceDriverType("tmux")
	sourceDriver, _ := NewSourceDriverType("git")
	a, _ := NewUnresolvedTemplate("a", "b", false, nil, nil, nil)
	b, _ := NewUnresolvedTemplate("b", "a", false, nil, nil, nil)
	config, _ := NewTemplates("sample", []Template{a, b}, workspaceDriver, None, sourceDriver, NoNotification)
	if _, err := config.Resolve("a", NewTemplateVars("", "", "", "")); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("cycle error = %v", err)
	}
	window, _ := NewWindow("agent", "")
	commander, _ := NewCommanderCellSpec("workspace", NewWorkspaceTemplate([]Window{window}))
	base, _ := NewUnresolvedTemplate("base", "", true, &commander, nil, nil)
	config, _ = NewTemplates("sample", []Template{base}, workspaceDriver, None, sourceDriver, NoNotification)
	if _, err := config.Resolve("base", NewTemplateVars("", "", "", "")); err == nil {
		t.Fatal("abstract template must not be selectable")
	}
	if _, err := config.Resolve("missing", NewTemplateVars("", "", "", "")); err == nil {
		t.Fatal("missing template must be rejected")
	}
}
