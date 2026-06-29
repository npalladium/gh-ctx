package resolve

import "testing"

func rules() []Rule {
	return []Rule{
		{GitDir: "~/npalladium/", Account: "npalladium", Source: "config.npalladium"},
		{GitDir: "~/org/", Account: "npalladium", Source: "config.npalladium"},
		{GitDir: "~/we360/", Account: "np-we360", Source: "config.we360"},
		{GitDir: "~/mavq/", Skip: true, Source: "config.mavq"},
	}
}

const home = "/home/n"

func TestMatch(t *testing.T) {
	tests := []struct {
		name    string
		dir     string
		want    string // account, "" with wantSkip, or "<nil>" for no match
		wantNil bool
	}{
		{"repo under work root", "/home/n/we360/api", "np-we360", false},
		{"deeply nested", "/home/n/we360/api/cmd/server", "np-we360", false},
		{"personal root exact", "/home/n/npalladium", "npalladium", false},
		{"second personal prefix", "/home/n/org/notes", "npalladium", false},
		{"skip tree matches with no account", "/home/n/mavq/thing", "", false},
		{"unmapped dir", "/home/n/random/project", "", true},
		{"prefix is not substring-fooled", "/home/n/we360-other/x", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Match(tt.dir, home, rules())
			if tt.wantNil {
				if got != nil {
					t.Fatalf("want nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("want account %q, got nil", tt.want)
			}
			if got.Account != tt.want {
				t.Fatalf("want account %q, got %q", tt.want, got.Account)
			}
		})
	}
}

func TestMatchLongestWins(t *testing.T) {
	rs := []Rule{
		{GitDir: "~/we360/", Account: "np-we360"},
		{GitDir: "~/we360/oss/", Account: "npalladium"}, // more specific subtree
	}
	got := Match("/home/n/we360/oss/proj", home, rs)
	if got == nil || got.Account != "npalladium" {
		t.Fatalf("longest prefix should win, got %+v", got)
	}
	got = Match("/home/n/we360/internal/proj", home, rs)
	if got == nil || got.Account != "np-we360" {
		t.Fatalf("shorter prefix should match outside subtree, got %+v", got)
	}
}

func TestMatchTrailingGlobEquivalentToSlash(t *testing.T) {
	a := Match("/home/n/we360/x", home, []Rule{{GitDir: "~/we360/", Account: "a"}})
	b := Match("/home/n/we360/x", home, []Rule{{GitDir: "~/we360/**", Account: "a"}})
	if a == nil || b == nil || a.Account != b.Account {
		t.Fatalf("trailing / and /** should match identically: %+v vs %+v", a, b)
	}
}

func TestMatchCaseInsensitive(t *testing.T) {
	rs := []Rule{{GitDir: "~/We360/", Account: "np-we360", CaseInsensitive: true}}
	if got := Match("/home/n/we360/api", home, rs); got == nil {
		t.Fatal("case-insensitive gitdir/i should match differing case")
	}
	rs[0].CaseInsensitive = false
	if got := Match("/home/n/we360/api", home, rs); got != nil {
		t.Fatal("case-sensitive gitdir should not match differing case")
	}
}

func TestMatchAbsolutePattern(t *testing.T) {
	rs := []Rule{{GitDir: "/srv/work/", Account: "work"}}
	if got := Match("/srv/work/repo", home, rs); got == nil || got.Account != "work" {
		t.Fatalf("absolute gitdir prefix should match, got %+v", got)
	}
}
