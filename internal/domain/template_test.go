package domain

import "testing"

func TestTemplatesは名前から各Templateを取得する(t *testing.T) {
	mode, err := NewMode("target")
	if err != nil {
		t.Fatal(err)
	}
	container, err := NewContainerTemplate("app", mode, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceTemplate(".", "main", "feat/")
	if err != nil {
		t.Fatal(err)
	}
	template, err := NewTemplate("feat", []SourceTemplate{source}, []ContainerTemplate{container}, NewSessionTemplate(nil))
	if err != nil {
		t.Fatal(err)
	}
	sessionDriver, _ := NewSessionDriverType("tmux")
	sourceDriver, _ := NewSourceDriverType("git")
	notificationDriver, _ := NewNotificationDriverType("")
	templates, err := NewTemplates("project", []Template{template}, sessionDriver, NewContainerDriverType("docker"), sourceDriver, notificationDriver)
	if err != nil {
		t.Fatal(err)
	}
	containers, err := templates.GetContainerTemplates("feat")
	if err != nil || len(containers) != 1 || containers[0].Name != "app" {
		t.Fatalf("containers = %#v, err = %v", containers, err)
	}
	if _, err := templates.GetSourceTemplates("missing"); err == nil {
		t.Fatal("missing template must fail")
	}
}

func TestNewContainerTemplateはDependencyの変更設定を拒否する(t *testing.T) {
	_, err := NewContainerTemplate("db", Dependency, []Environment{{Name: "A", Value: "B"}}, nil)
	if err == nil {
		t.Fatal("dependency environment must fail")
	}
}

func TestDriverTypeを生成する(t *testing.T) {
	if got := NewContainerDriverType("unknown"); got != None {
		t.Fatalf("container driver = %q", got)
	}
	if _, err := NewSessionDriverType(""); err == nil {
		t.Fatal("empty session driver must fail")
	}
	if _, err := NewSourceDriverType(""); err == nil {
		t.Fatal("empty source driver must fail")
	}
}
