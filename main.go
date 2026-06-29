// Command gh-ctx is a GitHub CLI extension that selects the right gh account
// per directory by deriving it from git's own `includeIf "gitdir:..."` rules,
// and scopes the choice to each shell via direnv (GH_TOKEN) instead of mutating
// global gh state. See README.md.
package main

import (
	"fmt"
	"os"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	args := os.Args[1:]
	cmd := "which"
	if len(args) > 0 {
		cmd = args[0]
		args = args[1:]
	}

	var err error
	switch cmd {
	case "which", "current":
		err = cmdWhich(args)
	case "env":
		err = cmdEnv(args)
	case "status":
		err = cmdStatus(args)
	case "install-direnv":
		err = cmdInstallDirenv(args)
	case "doctor":
		err = cmdDoctor(args)
	case "version", "--version", "-v":
		fmt.Println("gh-ctx", version)
	case "help", "--help", "-h":
		usage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "gh-ctx: unknown command %q\n\n", cmd)
		usage(os.Stderr)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gh-ctx:", err)
		os.Exit(1)
	}
}

func usage(w *os.File) {
	fmt.Fprint(w, `gh-ctx — per-directory gh account selection, derived from git includeIf.

Usage:
  gh ctx [which]          Show the gh account resolved for the current directory
  gh ctx env [DIR]        Print shell exports (GH_TOKEN/GH_HOST) for DIR (default: cwd)
  gh ctx status           Cross-check resolved account vs. active token vs. git email
  gh ctx install-direnv   Write the direnv glue and one managed .envrc per org root
  gh ctx doctor           Verify configuration and that mapped accounts are authed
  gh ctx version          Print version

Configuration lives in your git config. For each includeIf'd file, add:
  [gh-ctx]
      account = <gh-login>     # e.g. np-we360
      host    = github.com     # optional; omit for github.com
      skip    = true           # optional; mark a non-GitHub tree (e.g. GitLab)
`)
}
