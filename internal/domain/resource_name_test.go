package domain

import "testing"

func TestSafeResourceNameは利用できない文字をハイフンへ変換する(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "backslash", value: `feature\volume`, want: "feature-volume"},
		{name: "dockerで利用できない記号", value: "feature / copy:?*[test]", want: "feature-copy-test"},
		{name: "全角記号", value: "feature／volume＼copy", want: "feature-volume-copy"},
		{name: "安全な記号", value: "feature_name.v2-test", want: "feature_name.v2-test"},
		{name: "先頭と末尾の記号", value: "../feature/", want: "feature"},
		{name: "安全な文字がない", value: "日本語", want: "cell-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SafeResourceName(tt.value, "cell-1"); got != tt.want {
				t.Fatalf("SafeResourceName(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}
