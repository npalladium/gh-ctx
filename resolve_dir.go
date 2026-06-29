package main

import (
	"os"
	"path/filepath"

	"github.com/npalladium/gh-ctx/internal/ghcli"
	"github.com/npalladium/gh-ctx/internal/gitcfg"
	"github.com/npalladium/gh-ctx/internal/resolve"
)

// resolution is the outcome of mapping a directory to a gh account.
type resolution struct {
	Dir     string
	Account string
	Host    string // always concrete (defaults to github.com)
	Skip    bool
	Found   bool
	Via     string // "git" (in-repo, authoritative) or "path" (prefix fallback)
	Source  string // include file, when known
}

// resolveDir maps dir to an account. It first asks git itself (authoritative,
// honours all includeIf semantics) when dir is inside a repository, then falls
// back to matching dir against the includeIf gitdir prefixes from global
// config — which also works at org roots that are not themselves repos.
func resolveDir(dir string) (resolution, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return resolution{}, err
	}
	r := resolution{Dir: abs, Host: ghcli.DefaultHost}

	account, host, skip, found, err := gitcfg.DirSettings(abs)
	if err != nil {
		return r, err
	}
	if found {
		r.Account, r.Skip, r.Found, r.Via = account, skip, true, "git"
		if host != "" {
			r.Host = host
		}
		return r, nil
	}

	incs, err := gitcfg.Includes()
	if err != nil {
		return r, err
	}
	home, _ := os.UserHomeDir()
	if m := resolve.Match(abs, home, gitcfg.Rules(incs)); m != nil {
		r.Account, r.Skip, r.Found, r.Via, r.Source = m.Account, m.Skip, true, "path", m.Source
		if m.Host != "" {
			r.Host = m.Host
		}
	}
	return r, nil
}

func cwd() (string, error) { return os.Getwd() }
