// Package gitcfg reads the user's git configuration by shelling out to git, so
// that git itself remains the single source of truth (including all
// `includeIf` resolution semantics). gh-ctx never parses git config files by
// hand.
package gitcfg

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/npalladium/gh-ctx/internal/resolve"
)

// Include is one `includeIf "gitdir:..."` entry from the global git config,
// joined with the gh-ctx settings found in the file it points at.
type Include struct {
	GitDir          string // condition pattern, e.g. "~/we360/"
	CaseInsensitive bool   // gitdir/i: variant
	Path            string // the include file this rule points at
	Account         string // gh-ctx.account in that file ("" if unset)
	Host            string // gh-ctx.host in that file ("" => github.com)
	Skip            bool   // gh-ctx.skip = true (tree is not GitHub)
}

// Configured reports whether the include carries usable gh-ctx settings.
func (i Include) Configured() bool { return i.Account != "" || i.Skip }

// Includes enumerates every includeIf gitdir rule in the global git config and
// reads each target file's [gh-ctx] settings.
func Includes() ([]Include, error) {
	// -z gives NUL-separated records, each "key\nvalue", so values containing
	// '=' or newlines never confuse parsing.
	out, err := output("git", "config", "--global", "--list", "-z")
	if err != nil {
		return nil, err
	}
	var incs []Include
	for rec := range strings.SplitSeq(out, "\x00") {
		if rec == "" {
			continue
		}
		key, val, ok := strings.Cut(rec, "\n")
		if !ok {
			continue
		}
		cond, isPath := strings.CutPrefix(key, "includeif.")
		if !isPath {
			continue
		}
		cond, isPath = strings.CutSuffix(cond, ".path")
		if !isPath {
			continue
		}
		inc := Include{Path: val}
		switch {
		case strings.HasPrefix(cond, "gitdir/i:"):
			inc.GitDir = strings.TrimPrefix(cond, "gitdir/i:")
			inc.CaseInsensitive = true
		case strings.HasPrefix(cond, "gitdir:"):
			inc.GitDir = strings.TrimPrefix(cond, "gitdir:")
		default:
			continue // onbranch:, hasconfig:, etc. are not directory rules
		}
		inc.Account = fileValue(val, "gh-ctx.account")
		inc.Host = fileValue(val, "gh-ctx.host")
		inc.Skip = fileBool(val, "gh-ctx.skip")
		incs = append(incs, inc)
	}
	return incs, nil
}

// Rules converts configured includes into resolver rules (skipping includes
// that carry no gh-ctx settings).
func Rules(incs []Include) []resolve.Rule {
	var rs []resolve.Rule
	for _, i := range incs {
		if !i.Configured() {
			continue
		}
		rs = append(rs, resolve.Rule{
			GitDir:          i.GitDir,
			Account:         i.Account,
			Host:            i.Host,
			Skip:            i.Skip,
			CaseInsensitive: i.CaseInsensitive,
			Source:          i.Path,
		})
	}
	return rs
}

// DirSettings asks git to resolve gh-ctx settings for dir using git's own
// includeIf engine. found is true only when dir is inside a git repository and
// a value is configured. A missing value (exit status 1) is not an error.
func DirSettings(dir string) (account, host string, skip, found bool, err error) {
	account, err = optional("git", "-C", dir, "config", "--get", "gh-ctx.account")
	if err != nil {
		return "", "", false, false, err
	}
	host, err = optional("git", "-C", dir, "config", "--get", "gh-ctx.host")
	if err != nil {
		return "", "", false, false, err
	}
	skipStr, err := optional("git", "-C", dir, "config", "--get", "gh-ctx.skip")
	if err != nil {
		return "", "", false, false, err
	}
	skip = skipStr == "true"
	found = account != "" || skip
	return account, host, skip, found, nil
}

// Email returns user.email as git resolves it for dir ("" if unset/not a repo).
func Email(dir string) string {
	v, _ := optional("git", "-C", dir, "config", "--get", "user.email")
	return v
}

func fileValue(file, key string) string {
	v, _ := optional("git", "config", "--file", expandHome(file, homeDir()), "--get", key)
	return v
}

func fileBool(file, key string) bool {
	v, _ := optional("git", "config", "--file", expandHome(file, homeDir()), "--get", "--bool", key)
	return v == "true"
}

func homeDir() string { h, _ := os.UserHomeDir(); return h }

// expandHome resolves a leading "~" in an includeIf path against home. git
// expands "~" itself when resolving includes in-repo, but `git config --file`
// receives the raw value and (running without a shell) treats "~" literally, so
// gh-ctx must expand it before reading the included file directly.
func expandHome(p, home string) string {
	if home == "" {
		return p
	}
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}

// output runs a command and returns trimmed stdout, erroring on any failure.
func output(name string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", &CmdError{Name: name, Args: args, Stderr: stderr.String(), Err: err}
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

// optional runs a command that is expected to sometimes "fail" with exit
// status 1 to signal absence (git config --get conventions). Exit 1 yields
// ("", nil); any other failure is a real error.
func optional(name string, args ...string) (string, error) {
	s, err := output(name, args...)
	if err == nil {
		return s, nil
	}
	var ce *CmdError
	if errors.As(err, &ce) {
		var ee *exec.ExitError
		if errors.As(ce.Err, &ee) && ee.ExitCode() == 1 {
			return "", nil
		}
	}
	return "", err
}

// CmdError captures a failed external command for diagnostics.
type CmdError struct {
	Name   string
	Args   []string
	Stderr string
	Err    error
}

func (e *CmdError) Error() string {
	msg := e.Name + " " + strings.Join(e.Args, " ") + ": " + e.Err.Error()
	if s := strings.TrimSpace(e.Stderr); s != "" {
		msg += ": " + s
	}
	return msg
}

func (e *CmdError) Unwrap() error { return e.Err }
