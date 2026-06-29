package gitcfg

import "testing"

// Regression: includeIf paths commonly use "~" (e.g. "~/.config/git/config.x").
// git expands it during in-repo resolution, but `git config --file` gets the raw
// value with no shell, so gh-ctx must expand "~" before reading the file — else
// doctor/install-direnv silently see no [gh-ctx] settings.
func TestExpandHome(t *testing.T) {
	const home = "/home/n"
	tests := []struct{ in, want string }{
		{"~/.config/git/config.npalladium", "/home/n/.config/git/config.npalladium"},
		{"~", "/home/n"},
		{"/abs/path", "/abs/path"},
		{"relative/path", "relative/path"},
		{"~user/notme", "~user/notme"}, // only "~" and "~/" expand, not "~user"
	}
	for _, tt := range tests {
		if got := expandHome(tt.in, home); got != tt.want {
			t.Errorf("expandHome(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExpandHomeEmptyHomeIsNoop(t *testing.T) {
	if got := expandHome("~/x", ""); got != "~/x" {
		t.Errorf("with empty home, expandHome should be a no-op, got %q", got)
	}
}
