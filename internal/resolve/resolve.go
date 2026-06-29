// Package resolve maps a filesystem directory to a GitHub account by mirroring
// git's `includeIf "gitdir:..."` matching. It is pure logic with no I/O so it
// can be unit-tested in isolation; callers feed it the include rules they read
// from git config.
package resolve

import (
	"path/filepath"
	"sort"
	"strings"
)

// Rule is one resolved git includeIf entry plus the gh account it points at.
// GitDir is the raw condition pattern as written in git config (e.g.
// "~/we360/" or "/abs/path/"). Account is empty when Skip is true.
type Rule struct {
	GitDir          string // includeIf gitdir condition, verbatim
	Account         string // gh account login, e.g. "np-we360"
	Host            string // gh host; defaults to github.com when empty
	Skip            bool   // tree is intentionally not GitHub (e.g. GitLab)
	CaseInsensitive bool   // true for the gitdir/i: variant
	Source          string // path of the include file (for diagnostics)
}

// Match returns the most specific Rule whose gitdir pattern contains dir, or
// nil if none match. home is used to expand a leading "~/" in patterns. dir is
// expected to be absolute; it is cleaned before matching.
//
// Matching follows git's gitdir semantics for the cases gh-ctx generates and
// the overwhelmingly common hand-written case (a directory prefix ending in
// "/"). Full glob semantics (** in the middle, etc.) are intentionally not
// reimplemented — inside a repository gh-ctx defers to git itself, which is
// authoritative; this matcher is the fallback for non-repo lookups and for
// enumerating org roots.
func Match(dir, home string, rules []Rule) *Rule {
	dir = filepath.Clean(dir)

	type cand struct {
		rule *Rule
		// length of the matched base, longer == more specific
		score int
	}
	var cands []cand

	for i := range rules {
		r := &rules[i]
		base, ok := expand(r.GitDir, home)
		if !ok {
			continue
		}
		if matchPrefix(dir, base, r.CaseInsensitive) {
			cands = append(cands, cand{rule: r, score: len(base)})
		}
	}
	if len(cands) == 0 {
		return nil
	}
	// Longest base wins; stable on ties by original order.
	sort.SliceStable(cands, func(i, j int) bool {
		return cands[i].score > cands[j].score
	})
	return cands[0].rule
}

// BaseDir returns the concrete directory a gitdir pattern anchors to, with a
// leading "~/" expanded against home and any trailing "/**" or "/" removed. ok
// is false for patterns that are not plain directory prefixes.
func BaseDir(pattern, home string) (base string, ok bool) {
	return expand(pattern, home)
}

// expand turns a gitdir pattern into a concrete base directory for prefix
// matching. It handles the forms gh-ctx supports: leading "~/" (home),
// leading "./" is not meaningful without the include file's directory and is
// rejected, and a trailing "/" (or "/**") meaning "this dir and everything
// under it". The returned base never has a trailing slash. ok is false when
// the pattern is one we don't handle as a plain prefix.
func expand(pattern, home string) (base string, ok bool) {
	p := pattern
	// git appends /** when the pattern ends in /. Normalise both spellings.
	p = strings.TrimSuffix(p, "**")
	if strings.HasPrefix(p, "~/") {
		p = filepath.Join(home, p[2:])
	}
	// A bare relative pattern (no leading / or ~) has no stable anchor here.
	if !filepath.IsAbs(p) {
		return "", false
	}
	// Reject patterns with glob metacharacters in the middle; we only do
	// plain prefixes. A trailing component glob would have been stripped above.
	if strings.ContainsAny(p, "*?[") {
		return "", false
	}
	return strings.TrimRight(filepath.Clean(p), "/"), true
}

// matchPrefix reports whether dir is base or lies beneath base.
func matchPrefix(dir, base string, ci bool) bool {
	d, b := dir, base
	if ci {
		d, b = strings.ToLower(d), strings.ToLower(b)
	}
	if d == b {
		return true
	}
	return strings.HasPrefix(d, b+string(filepath.Separator))
}
