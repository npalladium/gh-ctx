// Package ghcli is a thin wrapper over the `gh` CLI. gh-ctx never touches the
// OS keyring directly; it asks gh, which owns secure credential storage.
package ghcli

import (
	"bytes"
	"os/exec"
	"regexp"
	"strings"
)

// DefaultHost is used whenever a rule does not pin an explicit host.
const DefaultHost = "github.com"

// Token returns the stored auth token for account on host without changing the
// active account (gh auth token --user). An empty host means the default host.
func Token(account, host string) (string, error) {
	args := []string{"auth", "token", "--user", account}
	if host != "" && host != DefaultHost {
		args = append(args, "--hostname", host)
	}
	return output("gh", args...)
}

// ActiveLogin returns the login that the current environment (honouring any
// GH_TOKEN/GH_HOST already exported) authenticates as.
func ActiveLogin() (string, error) {
	return output("gh", "api", "user", "--jq", ".login")
}

var accountLine = regexp.MustCompile(`account (\S+)`)

// Accounts lists the logins gh is currently authenticated with, parsed from
// `gh auth status`. Order is not significant.
func Accounts() ([]string, error) {
	// gh auth status writes the human report to stderr and exits non-zero when
	// any account has problems, so we capture both streams and ignore the code.
	var buf bytes.Buffer
	cmd := exec.Command("gh", "auth", "status")
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	_ = cmd.Run()
	var got []string
	seen := map[string]bool{}
	for _, m := range accountLine.FindAllStringSubmatch(buf.String(), -1) {
		if a := m[1]; !seen[a] {
			seen[a] = true
			got = append(got, a)
		}
	}
	return got, nil
}

func output(name string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if s := strings.TrimSpace(stderr.String()); s != "" {
			return "", &Error{Cmd: name + " " + strings.Join(args, " "), Stderr: s, Err: err}
		}
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Error wraps a failed gh invocation with its stderr.
type Error struct {
	Cmd    string
	Stderr string
	Err    error
}

func (e *Error) Error() string { return e.Cmd + ": " + e.Stderr }
func (e *Error) Unwrap() error { return e.Err }
