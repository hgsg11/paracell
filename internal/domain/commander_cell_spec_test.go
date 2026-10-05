package domain

import "testing"

func TestTargetAndDependencySpecsArePeers(t *testing.T) {
	source, _ := NewSourceTemplate(".", "main", "feat/")
	containerA, _ := NewContainerTemplate("web", Target, nil, nil)
	containerB, _ := NewContainerTemplate("worker", Target, nil, nil)
	if _, err := NewTargetCellSpec("empty", nil, nil); err == nil {
		t.Fatal("empty target accepted")
	}
	target, err := NewTargetCellSpec("app", &source, []ContainerTemplate{containerA, containerB})
	if err != nil || len(target.Containers) != 2 {
		t.Fatalf("target = %#v, err = %v", target, err)
	}
	if _, err := NewDependencyCellSpec("database"); err != nil {
		t.Fatal(err)
	}
	if _, err := NewDependencyCellSpec(""); err == nil {
		t.Fatal("empty dependency name accepted")
	}
}

func TestRuntimeTargetHasNoDependencyReferences(t *testing.T) {
	source, _ := NewSource(".", "main", "feat/api")
	if _, err := NewTargetCell("target-id", "group-id", "api", &source, nil); err != nil {
		t.Fatal(err)
	}
	container, _ := NewContainer(nil, "web", Target)
	if _, err := NewTargetCell("target-id", "group-id", "api", nil, []*Container{&container}); err != nil {
		t.Fatal(err)
	}
}
