package domain

import "testing"

func TestResolveTemplateはCommanderCell仕様とRuntime変数を解決する(t *testing.T) {
	source, _ := NewSourceTemplate("services/api", "origin/main")
	mode, _ := NewMode("target")
	environment, _ := NewEnvironment("CELL", "{{.Project}}-{{.Name}}")
	container, _ := NewContainerTemplate("api", mode, []Environment{environment}, nil)
	target, _ := NewTargetCellSpec("api", &source, []ContainerTemplate{container})
	dependency, _ := NewDependencyCellSpec("postgres")
	commander, err := NewCommanderCellSpec("workspace", NewWorkspaceTemplate([]Window{{Name: "agent", Command: "codex {{.Command}}"}}))
	if err != nil {
		t.Fatal(err)
	}
	feat, _ := NewUnresolvedTemplate("feat", &commander, []TargetCellSpec{target}, []DependencyCellSpec{dependency})
	workspaceDriver, _ := NewWorkspaceDriverType("tmux")
	sourceDriver, _ := NewSourceDriverType("git")
	config, _ := NewTemplates("sample", []Template{feat}, workspaceDriver, Docker, sourceDriver, NoNotification, nil)

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

func TestResolveTemplateは存在しないTemplateとCommanderCell未定義を拒否する(t *testing.T) {
	workspaceDriver, _ := NewWorkspaceDriverType("tmux")
	sourceDriver, _ := NewSourceDriverType("git")
	bare, _ := NewUnresolvedTemplate("bare", nil, nil, nil)
	config, _ := NewTemplates("sample", []Template{bare}, workspaceDriver, None, sourceDriver, NoNotification, nil)
	if _, err := config.Resolve("bare", NewTemplateVars("", "", "", "")); err == nil {
		t.Fatal("template without CommanderCell must be rejected")
	}
	if _, err := config.SelectableNames(); err == nil {
		t.Fatal("template without CommanderCell must not be selectable")
	}
	if _, err := config.Resolve("missing", NewTemplateVars("", "", "", "")); err == nil {
		t.Fatal("missing template must be rejected")
	}
}
