package domain

import "testing"

func TestWorkspaceResourceはprojectごとの専用socket名を返す(t *testing.T) {
	resource := NewWorkspaceResource("my-app-123", "123", "my app/api", "123", ".", nil)
	if got, want := resource.SocketName(), "paracell-my-app-api"; got != want {
		t.Fatalf("SocketName() = %q, want %q", got, want)
	}
	if got, want := WorkspaceSocketName(""), "paracell-project"; got != want {
		t.Fatalf("WorkspaceSocketName(\"\") = %q, want %q", got, want)
	}
}
