package direnv

import (
	"os"
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
