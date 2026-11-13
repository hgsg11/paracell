package domain

import (
	"reflect"
	"strings"
	"testing"
)

// newLayoutTestTarget builds a TargetCell. An empty sourcePath means no source.
func newLayoutTestTarget(t *testing.T, name string, sourcePath string, containers []string) TargetCell {
	t.Helper()
	var source *Source
	if sourcePath != "" {
		value, err := NewSource(sourcePath, "main", "feat/118")
		if err != nil {
			t.Fatal(err)
		}
		source = &value
	}
	items := make([]*Container, 0, len(containers))
	for _, name := range containers {
		container, err := NewContainer(nil, name, Target)
		if err != nil {
			t.Fatal(err)
		}
		items = append(items, &container)
	}
	target, err := NewTargetCell("id-"+name, "group", name, source, items)
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func TestCellResourceLayoutは衝突しない名前を今のまま使う(t *testing.T) {
	group := newTestCellGroup(t, "42")
	layout := BuildCellResourceLayoutService(group, []TargetCell{
		newLayoutTestTarget(t, "api", ".", []string{"app"}),
		newLayoutTestTarget(t, "worker", "services", []string{"app", "Cache"}),
	})
	checks := map[string]string{
		layout.TargetDirectory("api"):           ".paracell/cells/42/api",
		layout.SourceWorktreePath("api"):        ".paracell/cells/42/api/source",
		layout.SourceWorktreePath("worker"):     ".paracell/cells/42/worker/source/services",
		layout.ContainerName("api", "app"):      "paracell-My-App-42-api-app",
		layout.ContainerName("worker", "app"):   "paracell-My-App-42-worker-app",
		layout.ContainerName("worker", "Cache"): "paracell-My-App-42-worker-Cache",
		layout.ContainerRole("worker", "Cache"): "Cache",
		layout.SourceOwner("worker"):            "worker",
	}
	for got, want := range checks {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestCellResourceLayoutは大文字小文字と記号だけが違うターゲット名を区別する(t *testing.T) {
	group := newTestCellGroup(t, "42")
	layout := BuildCellResourceLayoutService(group, []TargetCell{
		newLayoutTestTarget(t, "API", "", []string{"app"}),
		newLayoutTestTarget(t, "api", "", []string{"app"}),
		newLayoutTestTarget(t, "my app", "", []string{"app"}),
		newLayoutTestTarget(t, "my-app", "", []string{"app"}),
		newLayoutTestTarget(t, "web", "", []string{"app"}),
	})
	seen := map[string]string{}
	for _, target := range []string{"API", "api", "my app", "my-app", "web"} {
		directory := strings.ToLower(layout.TargetDirectory(target))
		if other, exists := seen[directory]; exists {
			t.Fatalf("%q and %q share directory %q", other, target, directory)
		}
		seen[directory] = target
		name := strings.ToLower(layout.ContainerName(target, "app"))
		if other, exists := seen[name]; exists {
			t.Fatalf("%q and %q share container name %q", other, target, name)
		}
		seen[name] = target
	}
	if got := layout.TargetDirectory("API"); got != ".paracell/cells/42/API-"+shortHash("API") {
		t.Fatalf("colliding directory = %q", got)
	}
	if got := layout.TargetDirectory("web"); got != ".paracell/cells/42/web" {
		t.Fatalf("non-colliding directory = %q", got)
	}
}

func TestCellResourceLayoutはつなげると同じになるコンテナ名を区別する(t *testing.T) {
	group := newTestCellGroup(t, "42")
	layout := BuildCellResourceLayoutService(group, []TargetCell{
		newLayoutTestTarget(t, "a-b", "", []string{"c"}),
		newLayoutTestTarget(t, "a", "", []string{"b-c", "App", "app"}),
	})
	first, second := layout.ContainerName("a-b", "c"), layout.ContainerName("a", "b-c")
	if first == second {
		t.Fatalf("container names collide: %q", first)
	}
	if first != "paracell-My-App-42-a-b-c-"+shortHash("a-b/c") || second != "paracell-My-App-42-a-b-c-"+shortHash("a/b-c") {
		t.Fatalf("container names = %q / %q", first, second)
	}
	if strings.EqualFold(layout.ContainerName("a", "App"), layout.ContainerName("a", "app")) {
		t.Fatalf("case-only container names collide: %q", layout.ContainerName("a", "app"))
	}
}

func TestCellResourceLayoutは同じコンテナを使うターゲットのゲートウェイ名を分ける(t *testing.T) {
	group := newTestCellGroup(t, "42")
	layout := BuildCellResourceLayoutService(group, []TargetCell{
		newLayoutTestTarget(t, "api", "", []string{"app", "db"}),
		newLayoutTestTarget(t, "worker", "", []string{"app"}),
		newLayoutTestTarget(t, "web", "", []string{"App", "app"}),
	})
	if got := layout.ContainerRole("api", "db"); got != "db" {
		t.Fatalf("unique role = %q", got)
	}
	if got := layout.ContainerRole("api", "app"); got != "app.api" {
		t.Fatalf("api role = %q", got)
	}
	if got := layout.ContainerRole("worker", "app"); got != "app.worker" {
		t.Fatalf("worker role = %q", got)
	}
	upper, lower := layout.ContainerRole("web", "App"), layout.ContainerRole("web", "app")
	if strings.EqualFold(upper, lower) {
		t.Fatalf("roles in one target collide: %q / %q", upper, lower)
	}
}

func TestCellResourceLayoutは同じSourcePathのWorktreeを共有する(t *testing.T) {
	group := newTestCellGroup(t, "42")
	layout := BuildCellResourceLayoutService(group, []TargetCell{
		newLayoutTestTarget(t, "worker", ".", nil),
		newLayoutTestTarget(t, "api", ".", nil),
		newLayoutTestTarget(t, "docs", "docs", nil),
	})
	if got := layout.SourceOwner("worker"); got != "api" {
		t.Fatalf("owner = %q", got)
	}
	if got := layout.SourceWorktreePath("worker"); got != ".paracell/cells/42/api/source" {
		t.Fatalf("shared worktree = %q", got)
	}
	if got := layout.SourceRoot("worker"); got != ".paracell/cells/42/worker/source" {
		t.Fatalf("link path = %q", got)
	}
	if got := layout.SourceOwner("docs"); got != "docs" {
		t.Fatalf("docs owner = %q", got)
	}
}

func TestCellResourceLayoutはターゲットの順番に左右されない(t *testing.T) {
	group := newTestCellGroup(t, "42")
	targets := []TargetCell{
		newLayoutTestTarget(t, "API", ".", []string{"app"}),
		newLayoutTestTarget(t, "api", ".", []string{"app"}),
		newLayoutTestTarget(t, "a-b", "", []string{"c"}),
		newLayoutTestTarget(t, "a", "", []string{"b-c"}),
	}
	reversed := []TargetCell{targets[3], targets[2], targets[1], targets[0]}
	if first, second := BuildCellResourceLayoutService(group, targets), BuildCellResourceLayoutService(group, reversed); !reflect.DeepEqual(first, second) {
		t.Fatalf("layout depends on order:\n%#v\n%#v", first, second)
	}
}

func TestBuildSourceResourcesServiceは同じSourcePathをリンクで共有する(t *testing.T) {
	group := newTestCellGroup(t, "42")
	resources := BuildSourceResourcesService(group, []TargetCell{
		newLayoutTestTarget(t, "worker", ".", nil),
		newLayoutTestTarget(t, "api", ".", nil),
		newLayoutTestTarget(t, "db", "", []string{"db"}),
	})
	want := []SourceResource{{
		RepositoryPath: ".", WorktreePath: ".paracell/cells/42/api/source", Base: "main", Branch: "feat/118",
		Links: []SourceLink{{Path: ".paracell/cells/42/worker/source", Target: ".paracell/cells/42/api/source"}},
	}}
	if !reflect.DeepEqual(resources, want) {
		t.Fatalf("resources = %#v", resources)
	}
}

func TestBuildContainerResourcesServiceはターゲットごとの設定を使う(t *testing.T) {
	group := newTestCellGroup(t, "42")
	targets := []TargetCell{
		newLayoutTestTarget(t, "api", ".", []string{"app"}),
		newLayoutTestTarget(t, "worker", ".", []string{"app"}),
	}
	templates := map[ContainerTemplateKey]ContainerTemplate{
		{Target: "api", Container: "app"}:    {Name: "app", Mode: Target, Environments: []Environment{{Name: "ROLE", Value: "api"}}},
		{Target: "worker", Container: "app"}: {Name: "app", Mode: Target, Environments: []Environment{{Name: "ROLE", Value: "worker"}}},
	}
	resources := BuildContainerResourcesService(group, targets, nil, templates)
	if len(resources.Items) != 2 {
		t.Fatalf("items = %#v", resources.Items)
	}
	for _, item := range resources.Items {
		if len(item.Environments) != 1 || item.Environments[0].Value != item.Target {
			t.Fatalf("target %q got environments %#v", item.Target, item.Environments)
		}
		if item.SourcePath != ".paracell/cells/42/api/source" {
			t.Fatalf("target %q source path = %q", item.Target, item.SourcePath)
		}
		if item.Role != "app."+item.Target {
			t.Fatalf("target %q role = %q", item.Target, item.Role)
		}
	}
}

func TestNewContainerTemplateは正規化すると同じマウントをまとめる(t *testing.T) {
	first, err := NewMount("/data/", "data")
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewMount("/data", "./data")
	if err != nil {
		t.Fatal(err)
	}
	template, err := NewContainerTemplate("app", Target, nil, []Mount{first, second})
	if err != nil {
		t.Fatal(err)
	}
	if want := []Mount{{TargetPath: "/data", SourcePath: "data"}}; !reflect.DeepEqual(template.Mounts, want) {
		t.Fatalf("mounts = %#v", template.Mounts)
	}
	other, err := NewMount("/data", "other")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewContainerTemplate("app", Target, nil, []Mount{first, other}); err == nil {
		t.Fatal("different sources at the same path were accepted")
	}
}

func TestNewUnresolvedTemplateは安全な名前が同じターゲットを受け付ける(t *testing.T) {
	source, err := NewSourceTemplate(".", "main")
	if err != nil {
		t.Fatal(err)
	}
	first, err := NewTargetCellSpec("API", &source, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewTargetCellSpec("api", &source, nil)
	if err != nil {
		t.Fatal(err)
	}
	third, err := NewTargetCellSpec("a p i", &source, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewUnresolvedTemplate("feat", nil, []TargetCellSpec{first, second, third}, nil); err != nil {
		t.Fatal(err)
	}
}
