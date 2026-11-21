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
		layout.CellDirectory():                  ".paracell/cells/42",
		layout.SourceWorktreePath("api"):        ".paracell/cells/42",
		layout.SourceWorktreePath("worker"):     ".paracell/cells/42/services",
		layout.ContainerName("api", "app"):      "paracell-My-App-42-api-app",
		layout.ContainerName("worker", "app"):   "paracell-My-App-42-worker-app",
		layout.ContainerName("worker", "Cache"): "paracell-My-App-42-worker-Cache",
		layout.ContainerRole("worker", "Cache"): "Cache",
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
		name := strings.ToLower(layout.ContainerName(target, "app"))
		if other, exists := seen[name]; exists {
			t.Fatalf("%q and %q share container name %q", other, target, name)
		}
		seen[name] = target
	}
	if got := layout.ContainerName("API", "app"); got != "paracell-My-App-42-API-"+shortHash("API")+"-app" {
		t.Fatalf("colliding container name = %q", got)
	}
	if got := layout.ContainerName("web", "app"); got != "paracell-My-App-42-web-app" {
		t.Fatalf("non-colliding container name = %q", got)
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
	if worker, api := layout.SourceWorktreePath("worker"), layout.SourceWorktreePath("api"); worker != ".paracell/cells/42" || api != worker {
		t.Fatalf("shared worktree = %q / %q", worker, api)
	}
	if got := layout.SourceWorktreePath("docs"); got != ".paracell/cells/42/docs" {
		t.Fatalf("docs worktree = %q", got)
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

func TestBuildSourceResourcesServiceは同じSourcePathのWorktreeを1つだけ作る(t *testing.T) {
	group := newTestCellGroup(t, "42")
	resources := BuildSourceResourcesService(group, []TargetCell{
		newLayoutTestTarget(t, "worker", ".", nil),
		newLayoutTestTarget(t, "api", ".", nil),
		newLayoutTestTarget(t, "db", "", []string{"db"}),
	})
	want := []SourceResource{{RepositoryPath: ".", WorktreePath: ".paracell/cells/42", Base: "main", Branch: "feat/118"}}
	if !reflect.DeepEqual(resources, want) {
		t.Fatalf("resources = %#v", resources)
	}
}

func TestBuildSourceResourcesServiceはSourcePathをワークスペースと同じ場所に置く(t *testing.T) {
	group := newTestCellGroup(t, "42")
	resources := BuildSourceResourcesService(group, []TargetCell{
		newLayoutTestTarget(t, "frontend", "frontend", nil),
		newLayoutTestTarget(t, "backend", "backend", nil),
	})
	want := []SourceResource{
		{RepositoryPath: "backend", WorktreePath: ".paracell/cells/42/backend", Base: "main", Branch: "feat/118"},
		{RepositoryPath: "frontend", WorktreePath: ".paracell/cells/42/frontend", Base: "main", Branch: "feat/118"},
	}
	if !reflect.DeepEqual(resources, want) {
		t.Fatalf("resources = %#v", resources)
	}
}

func TestBuildSourceResourcesServiceは浅いSourcePathから順に返す(t *testing.T) {
	group := newTestCellGroup(t, "42")
	resources := BuildSourceResourcesService(group, []TargetCell{
		newLayoutTestTarget(t, "deep", "services/api/v2", nil),
		newLayoutTestTarget(t, "api", "services/api", nil),
		newLayoutTestTarget(t, "root", ".", nil),
		newLayoutTestTarget(t, "web", "web", nil),
	})
	var got []string
	for _, resource := range resources {
		got = append(got, resource.WorktreePath)
	}
	want := []string{".paracell/cells/42", ".paracell/cells/42/web", ".paracell/cells/42/services/api", ".paracell/cells/42/services/api/v2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("worktrees = %v, want %v", got, want)
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
	if resources.CellDirectory != ".paracell/cells/42" {
		t.Fatalf("cell directory = %q", resources.CellDirectory)
	}
	if len(resources.Items) != 2 {
		t.Fatalf("items = %#v", resources.Items)
	}
	for _, item := range resources.Items {
		if len(item.Environments) != 1 || item.Environments[0].Value != item.Target {
			t.Fatalf("target %q got environments %#v", item.Target, item.Environments)
		}
		if item.SourcePath != ".paracell/cells/42" {
			t.Fatalf("target %q source path = %q", item.Target, item.SourcePath)
		}
		if item.Role != "app."+item.Target {
			t.Fatalf("target %q role = %q", item.Target, item.Role)
		}
	}
}

func TestBuildContainerResourcesServiceはSourcePathのWorktreeとCellDirectoryを渡す(t *testing.T) {
	group := newTestCellGroup(t, "42")
	resources := BuildContainerResourcesService(group, []TargetCell{
		newLayoutTestTarget(t, "frontend", "frontend", []string{"web"}),
	}, nil, nil)
	if resources.CellDirectory != ".paracell/cells/42" || resources.Items[0].SourcePath != ".paracell/cells/42/frontend" {
		t.Fatalf("cell directory = %q, source path = %q", resources.CellDirectory, resources.Items[0].SourcePath)
	}
	if got := resources.Items[0].Name; got != "paracell-My-App-42-frontend-web" {
		t.Fatalf("container name = %q", got)
	}
	without := BuildContainerResourcesService(group, []TargetCell{newLayoutTestTarget(t, "db", "", []string{"db"})}, nil, nil)
	if without.CellDirectory != "" {
		t.Fatalf("cell directory without sources = %q", without.CellDirectory)
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
