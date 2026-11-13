package direnv

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertBlockAppendThenReplaceIdempotent(t *testing.T) {
	orig := "export FOO=1\n"
	out1, changed := upsertBlock(orig, "use gh_ctx a")
	if !changed {
		t.Fatal("first insert should change content")
	}
	if !strings.Contains(out1, "export FOO=1") {
		t.Fatal("existing content must be preserved")
	}
	if !strings.Contains(out1, "use gh_ctx a") {
		t.Fatal("block body missing")
	}
	// Re-applying identical body is a no-op.
	out2, changed := upsertBlock(out1, "use gh_ctx a")
	if changed || out2 != out1 {
		t.Fatal("re-applying same block must be idempotent")
	}
	// Changing the body replaces only the block, keeping surroundings.
	out3, changed := upsertBlock(out1, "use gh_ctx b")
	if !changed {
		t.Fatal("changed body should update")
	}
	if !strings.Contains(out3, "export FOO=1") || !strings.Contains(out3, "use gh_ctx b") || strings.Contains(out3, "use gh_ctx a") {
		t.Fatalf("block not cleanly replaced: %q", out3)
	}
	if strings.Count(out3, begin) != 1 {
		t.Fatal("must not duplicate markers")
	}
}

func TestWriteEnvrcRefusesUnmanagedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".envrc")
	const user = "layout python\nexport SECRET=keep-me\n"
	if err := os.WriteFile(path, []byte(user), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := WriteEnvrc(dir, "use gh_ctx a")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != Unmanaged {
		t.Fatalf("must not touch unmanaged .envrc, got %v", res.Status)
	}
	got, _ := os.ReadFile(path)
	if string(got) != user {
		t.Fatal("unmanaged .envrc content must be left byte-for-byte intact")
	}
}

func TestWriteEnvrcCreateUpdateUnchanged(t *testing.T) {
	dir := t.TempDir()
	res, err := WriteEnvrc(dir, "use gh_ctx a")
	if err != nil || res.Status != Created {
		t.Fatalf("first write should create, got %v err %v", res.Status, err)
	}
	res, _ = WriteEnvrc(dir, "use gh_ctx a")
	if res.Status != Unchanged {
		t.Fatalf("identical rewrite should be unchanged, got %v", res.Status)
	}
	res, _ = WriteEnvrc(dir, "use gh_ctx b")
	if res.Status != Updated {
		t.Fatalf("body change should update, got %v", res.Status)
	}
	b, _ := os.ReadFile(filepath.Join(dir, ".envrc"))
	if !strings.Contains(string(b), "use gh_ctx b") {
		t.Fatal("updated body not written")
	}
}

func TestEnvrcBodySkipClearsEnv(t *testing.T) {
	body := EnvrcBody("", "", true)
	if !strings.Contains(body, "unset GH_TOKEN") || !strings.Contains(body, "unset GH_HOST") {
		t.Fatalf("skip body must clear gh env, got %q", body)
	}
}

// Other tools may keep their own entries both above and below the gh-ctx block.
// Regenerating must rewrite only our block, leave both neighbours byte-for-byte
// intact, and be a no-op when the body is unchanged.
func TestUpsertBlockPreservesSurroundingToolEntries(t *testing.T) {
	before := "# asdf\nuse asdf\n\n"
	after := "\n# some other tool\nlayout python\n"
	orig := before + begin + "\n" + "use gh_ctx a" + "\n" + end + after

	// Same body: no change.
	out, changed := upsertBlock(orig, "use gh_ctx a")
	if changed || out != orig {
		t.Fatalf("re-applying same block must be a no-op; changed=%v\n%q", changed, out)
	}

	// Changed body: only our block is rewritten; both neighbours survive.
	out, changed = upsertBlock(orig, "use gh_ctx b")
	if !changed {
		t.Fatal("changed body should update")
	}
	if !strings.HasPrefix(out, before) {
		t.Fatalf("content before block not preserved: %q", out)
	}
	if !strings.HasSuffix(out, after) {
		t.Fatalf("content after block not preserved: %q", out)
	}
	if strings.Count(out, begin) != 1 || strings.Count(out, end) != 1 {
		t.Fatalf("must keep exactly one marker pair: %q", out)
	}
	if strings.Contains(out, "use gh_ctx a") {
		t.Fatalf("old body should be gone: %q", out)
	}
}

func TestRcFunctionAccountSelection(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal("bash is required to exercise the direnv integration")
	}
	for _, tt := range []struct {
		name string
		mode string
		host string
		want string
	}{
		{"default host", "success", "", "0|selected-token|github.com"},
		{"enterprise host", "success", "github.example.com", "0|selected-token|github.example.com"},
		{"missing token", "missing", "", "1|unset|unset"},
		{"empty token", "empty", "", "1|unset|unset"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			script := RcFunction + `
log_error() { printf '%s\n' "$*" >&2; }
gh() {
  [ "$1" = auth ] && [ "$2" = token ] &&
    [ "$3" = --user ] && [ "$4" = requested-account ] &&
    [ "$5" = --hostname ] && [ "$6" = "${TEST_HOST:-github.com}" ] || return 2
  case "$TEST_MODE" in
    success) printf '%s\n' selected-token ;;
    missing) return 1 ;;
    empty) return 0 ;;
  esac
}
export GH_TOKEN=inherited-token GH_HOST=previous.example.com
use_gh_ctx requested-account "${TEST_HOST:-github.com}"
result=$?
printf '%s|%s|%s' "$result" "${GH_TOKEN-unset}" "${GH_HOST-unset}"
`
			cmd := exec.Command(bash, "--noprofile", "--norc", "-c", script)
			cmd.Env = []string{"HOME=" + t.TempDir(), "TEST_MODE=" + tt.mode, "TEST_HOST=" + tt.host}
			var stderr strings.Builder
			cmd.Stderr = &stderr
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("shell failed: %v: %s", err, stderr.String())
			}
			if string(out) != tt.want {
				t.Fatalf("account selection = %q, want %q", out, tt.want)
			}
			if tt.mode == "success" {
				if stderr.Len() != 0 {
					t.Fatalf("successful selection logged an error: %s", stderr.String())
				}
			} else if !strings.Contains(stderr.String(), "no gh token") {
				t.Fatalf("missing authentication diagnostic: %s", stderr.String())
			}
			if strings.Contains(stderr.String(), "inherited-token") || strings.Contains(stderr.String(), "selected-token") {
				t.Fatal("authentication diagnostic exposed a token")
			}
		})
	}
}

func TestRcFunctionStopsFailedEnvironmentLoad(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal("bash is required to exercise the direnv integration")
	}
	script := RcFunction + `
set -e
log_error() { :; }
gh() { return 1; }
export GH_TOKEN=inherited-token GH_HOST=previous.example.com
use_gh_ctx requested-account
printf 'environment-loaded'
`
	cmd := exec.Command(bash, "--noprofile", "--norc", "-c", script)
	cmd.Env = []string{"HOME=" + t.TempDir()}
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("failed authentication must reject the environment load")
	}
	if len(out) != 0 {
		t.Fatalf("environment continued after failed authentication: %q", out)
	}
}
