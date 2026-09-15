package domain

import (
	"strings"
	"testing"
)

func TestResolveTemplateは継承とRuntime変数展開を担当する(t *testing.T) {
	baseSource, _ := NewSourceTemplate(".", "origin/main", "")
	baseSession := NewSessionTemplate([]Window{{Name: "agent", Command: "codex {{.Command}}"}})
	base, _ := NewUnresolvedTemplate("base", "", true, &baseSource, nil, &baseSession)
	prefix := "feat/"
	childSource, _ := NewPartialSourceTemplate(nil, nil, &prefix)
	environment, _ := NewEnvironment("CELL", "{{.Project}}-{{.Name}}")
	container, _ := NewContainerTemplate("app", Target, []Environment{environment}, nil)
	containers := []ContainerTemplate{container}
	child, _ := NewUnresolvedTemplate("feat", "base", false, &childSource, &containers, nil)
	sessionDriver, _ := NewSessionDriverType("tmux")
	sourceDriver, _ := NewSourceDriverType("git")
	config, _ := NewTemplates("sample", []Template{base, child}, sessionDriver, Docker, sourceDriver, NoNotification)

	resolved, err := ResolveTemplate(config, "feat", NewTemplateVars("42", "42", "sample", "work"))
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Sources[0].Base != "origin/main" || resolved.Sources[0].Prefix != "feat/" {
		t.Fatalf("source = %#v", resolved.Sources[0])
	}
	if resolved.Containers[0].Environments[0].Value != "sample-42" {
		t.Fatalf("environment = %#v", resolved.Containers[0].Environments)
	}
	if resolved.Session.Windows[0].Command != "codex work" {
		t.Fatalf("session = %#v", resolved.Session)
	}
}

func TestResolveTemplateは不正な継承と式を拒否する(t *testing.T) {
	sessionDriver, _ := NewSessionDriverType("tmux")
	sourceDriver, _ := NewSourceDriverType("git")
	a, _ := NewUnresolvedTemplate("a", "b", false, nil, nil, nil)
	b, _ := NewUnresolvedTemplate("b", "a", false, nil, nil, nil)
	config, _ := NewTemplates("sample", []Template{a, b}, sessionDriver, None, sourceDriver, NoNotification)
	if _, err := ResolveTemplate(config, "a", NewTemplateVars("", "", "", "")); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("cycle error = %v", err)
	}

	window, _ := NewWindow("agent", "{{.Missing}}")
	bad, _ := NewTemplate("bad", nil, nil, NewSessionTemplate([]Window{window}))
	config, _ = NewTemplates("sample", []Template{bad}, sessionDriver, None, sourceDriver, NoNotification)
	if _, err := ResolveTemplate(config, "bad", NewTemplateVars("", "", "", "")); err == nil {
		t.Fatal("undefined variable must fail")
	}
}

func TestDriverTypeを生成する(t *testing.T) {
	if NewContainerDriverType("unknown") != None {
		t.Fatal("unknown container driver must become none")
	}
	if _, err := NewSessionDriverType(""); err == nil {
		t.Fatal("empty session driver must fail")
	}
	if _, err := NewSourceDriverType(""); err == nil {
		t.Fatal("empty source driver must fail")
	}
}
