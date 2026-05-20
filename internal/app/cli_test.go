package app

import "testing"

func TestCreateコマンドを解析できる(t *testing.T) {
	cmd, err := ParseCommand([]string{"create", "123", "--template", "webapp"})

	if err != nil {
		t.Fatalf("create解析でエラーが返った: %v", err)
	}
	if cmd.Kind != CommandCreate {
		t.Fatalf("command kind = %q, want %q", cmd.Kind, CommandCreate)
	}
	if cmd.Issue != "123" {
		t.Fatalf("issue = %q, want %q", cmd.Issue, "123")
	}
	if cmd.Template != "webapp" {
		t.Fatalf("template = %q, want %q", cmd.Template, "webapp")
	}
}

func TestRemoveコマンドを解析できる(t *testing.T) {
	cmd, err := ParseCommand([]string{"remove", "123", "--force"})

	if err != nil {
		t.Fatalf("remove解析でエラーが返った: %v", err)
	}
	if cmd.Kind != CommandRemove {
		t.Fatalf("command kind = %q, want %q", cmd.Kind, CommandRemove)
	}
	if cmd.Cell != "123" {
		t.Fatalf("cell = %q, want %q", cmd.Cell, "123")
	}
	if !cmd.Force {
		t.Fatal("force = false, want true")
	}
}

func Test未対応コマンドはエラーにする(t *testing.T) {
	_, err := ParseCommand([]string{"list"})

	if err == nil {
		t.Fatal("未対応コマンドなのにエラーが返らなかった")
	}
}
