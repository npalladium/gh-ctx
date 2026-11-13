// Package direnv generates the direnv glue gh-ctx relies on: a use_gh_ctx
// stdlib function in direnvrc, and one managed .envrc per org root. All writes
// are idempotent and never destroy unmanaged content.
package direnv

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	begin = "# >>> gh-ctx >>>"
	end   = "# <<< gh-ctx <<<"
)

// RcFunction is the use_gh_ctx helper installed into direnvrc. `use gh_ctx
// <account> [host]` in an .envrc invokes it.
const RcFunction = `# Managed by gh-ctx (https://github.com/npalladium/gh-ctx).
# Do not edit between the gh-ctx markers; regenerate with: gh ctx install-direnv
use_gh_ctx() {
  local account="$1" host="${2:-github.com}"
  [ -n "$account" ] || return 0
  local token
  if ! token="$(gh auth token --user "$account" --hostname "$host" 2>/dev/null)" || [ -z "$token" ]; then
    unset GH_TOKEN GH_HOST
    log_error "gh-ctx: no gh token for '$account' on $host (try: gh auth login --hostname $host --user $account)"
    return 1
  fi
  export GH_HOST="$host"
  export GH_TOKEN="$token"
}`

// RcPath returns the direnvrc path honouring XDG_CONFIG_HOME.
func RcPath() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "direnv", "direnvrc")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "direnv", "direnvrc")
}

// EnvrcBody returns the managed body (without markers) for an org-root .envrc.
func EnvrcBody(account, host string, skip bool) string {
	if skip {
		return "# gh-ctx: this tree is not GitHub; clear any inherited gh credentials\n" +
			"unset GH_TOKEN\nunset GH_HOST"
	}
	line := "use gh_ctx " + account
	if host != "" && host != "github.com" {
		line += " " + host
	}
	return line
}

// WriteRc ensures the use_gh_ctx function exists in direnvrc, replacing a prior
// gh-ctx block if present and otherwise appending. Existing unrelated content
// is preserved. Returns whether the file changed.
func WriteRc() (path string, changed bool, err error) {
	path = RcPath()
	existing := readFile(path)
	updated, changed := upsertBlock(existing, RcFunction)
	if !changed {
		return path, false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return path, false, err
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return path, false, err
	}
	return path, true, nil
}

// WriteEnvrc writes a managed .envrc at dir. It refuses to overwrite an
// existing .envrc that gh-ctx does not manage (no gh-ctx markers), returning
// Unmanaged so the caller can warn instead of clobbering. When the file
// already contains a gh-ctx block, only that block is updated.
func WriteEnvrc(dir, body string) (Result, error) {
	path := filepath.Join(dir, ".envrc")
	existing, hadFile := readFileOK(path)
	block := begin + "\n" + body + "\n" + end + "\n"

	if hadFile && !strings.Contains(existing, begin) {
		return Result{Path: path, Status: Unmanaged}, nil
	}
	var content string
	if hadFile {
		content, _ = upsertBlock(existing, body)
	} else {
		content = block
	}
	if hadFile && content == existing {
		return Result{Path: path, Status: Unchanged}, nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return Result{Path: path, Status: Failed}, err
	}
	st := Created
	if hadFile {
		st = Updated
	}
	return Result{Path: path, Status: st}, nil
}

// Status describes the outcome of a managed-file write.
type Status int

const (
	Created   Status = iota // file did not exist, written
	Updated                 // gh-ctx block replaced in an existing managed file
	Unchanged               // already up to date
	Unmanaged               // existing file without gh-ctx markers; left untouched
	Failed
)

func (s Status) String() string {
	switch s {
	case Created:
		return "created"
	case Updated:
		return "updated"
	case Unchanged:
		return "unchanged"
	case Unmanaged:
		return "skipped (existing, unmanaged)"
	default:
		return "failed"
	}
}

// Result reports what happened to one path.
type Result struct {
	Path   string
	Status Status
}

// upsertBlock replaces the content between gh-ctx markers with body, or appends
// a fresh block when no markers are present. Returns the new text and whether
// it differs from the input.
func upsertBlock(existing, body string) (string, bool) {
	block := begin + "\n" + body + "\n" + end
	i := strings.Index(existing, begin)
	j := strings.Index(existing, end)
	if i >= 0 && j > i {
		before := existing[:i]
		after := existing[j+len(end):]
		updated := before + block + after
		return updated, updated != existing
	}
	var b strings.Builder
	b.WriteString(existing)
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		b.WriteString("\n")
	}
	if existing != "" {
		b.WriteString("\n")
	}
	b.WriteString(block)
	b.WriteString("\n")
	return b.String(), true
}

func readFile(path string) string { s, _ := readFileOK(path); return s }

func readFileOK(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(b), true
}

// AllowHint returns the command a user runs to trust a generated .envrc.
func AllowHint(dir string) string { return fmt.Sprintf("direnv allow %s", dir) }
