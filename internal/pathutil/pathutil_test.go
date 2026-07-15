package pathutil

import (
	"path/filepath"
	"testing"
)

func TestExpandTilde(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)        // unix
	t.Setenv("USERPROFILE", home) // windows

	tests := []struct {
		in, want string
	}{
		{"~", home},
		{"~/", home},
		{"~/clips/party.gif", filepath.Join(home, "clips", "party.gif")},
		{`~\clips`, filepath.Join(home, "clips")},
		{"plain/path.gif", "plain/path.gif"},
		{"~otheruser/x.gif", "~otheruser/x.gif"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := ExpandTilde(tt.in); got != tt.want {
			t.Errorf("ExpandTilde(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
