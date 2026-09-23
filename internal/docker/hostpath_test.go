package docker

import (
	"os"
	"testing"
)

func TestTranslateToHost(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}

	cases := []struct {
		name      string
		workspace string
		homeHost  string
		in        string
		want      string
	}{
		{"empty input", "/h/work", "/h", "", ""},
		{"no env vars: passthrough", "", "", "/workspace/foo", "/workspace/foo"},
		{"workspace exact", "/h/work", "", "/workspace", "/h/work"},
		{"workspace child", "/h/work", "", "/workspace/foo/bar", "/h/work/foo/bar"},
		{"workspace boundary not matched on partial segment", "/h/work", "", "/workspaceful/foo", "/workspaceful/foo"},
		{"home exact", "", "/h", home, "/h"},
		{"home child", "", "/h", home + "/.claude", "/h/.claude"},
		{"workspace wins when both prefixes overlap", "/host/work", "/host/home", "/workspace/x", "/host/work/x"},
		{"path outside both prefixes", "/h/work", "/h", "/etc/hosts", "/etc/hosts"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envHostWorkspace, tc.workspace)
			t.Setenv(envHostHome, tc.homeHost)
			if got := TranslateToHost(tc.in); got != tc.want {
				t.Errorf("TranslateToHost(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestTranslateToHost_UnsetEnvNoOp(t *testing.T) {
	t.Setenv(envHostWorkspace, "")
	t.Setenv(envHostHome, "")
	in := "/workspace/foo"
	if got := TranslateToHost(in); got != in {
		t.Errorf("TranslateToHost(%q) = %q; want passthrough", in, got)
	}
}
