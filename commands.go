package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/npalladium/gh-ctx/internal/direnv"
	"github.com/npalladium/gh-ctx/internal/ghcli"
	"github.com/npalladium/gh-ctx/internal/gitcfg"
	"github.com/npalladium/gh-ctx/internal/resolve"
)

// cmdWhich prints the account resolved for the current directory.
func cmdWhich(_ []string) error {
	dir, err := cwd()
	if err != nil {
		return err
	}
	r, err := resolveDir(dir)
	if err != nil {
		return err
	}
	switch {
	case !r.Found:
		fmt.Printf("no gh-ctx mapping for %s\n", r.Dir)
		fmt.Println("(add [gh-ctx] account to the includeIf'd git config for this tree)")
	case r.Skip:
		fmt.Printf("%s → (not GitHub; gh env cleared) [via %s]\n", r.Dir, r.Via)
	default:
		fmt.Printf("%s → %s on %s [via %s]\n", r.Dir, r.Account, r.Host, r.Via)
	}
	return nil
}

// cmdEnv prints shell exports for DIR (default cwd). Output is eval-safe: on no
// match it prints nothing to stdout and exits 0, so `eval "$(gh ctx env)"` is
// harmless in unmapped directories.
func cmdEnv(args []string) error {
	dir, err := cwd()
	if err != nil {
		return err
	}
	if len(args) > 0 {
		dir = args[0]
	}
	r, err := resolveDir(dir)
	if err != nil {
		return err
	}
	if !r.Found {
		fmt.Fprintf(os.Stderr, "gh-ctx: no mapping for %s\n", r.Dir)
		return nil
	}
	if r.Skip {
		fmt.Println("unset GH_TOKEN GH_HOST")
		return nil
	}
	token, err := ghcli.Token(r.Account, r.Host)
	if err != nil {
		return fmt.Errorf("no token for %q on %s (try: gh auth login --hostname %s): %w", r.Account, r.Host, r.Host, err)
	}
	fmt.Printf("export GH_HOST=%s\n", shellQuote(r.Host))
	fmt.Printf("export GH_TOKEN=%s\n", shellQuote(token))
	return nil
}

// cmdStatus cross-checks the resolved account against the active token's login
// and the git email for the current directory, surfacing drift.
func cmdStatus(_ []string) error {
	dir, err := cwd()
	if err != nil {
		return err
	}
	r, err := resolveDir(dir)
	if err != nil {
		return err
	}
	fmt.Printf("directory : %s\n", r.Dir)
	if !r.Found {
		fmt.Println("mapping   : none — run `gh ctx install-direnv` after adding [gh-ctx] settings")
		return nil
	}
	if r.Skip {
		fmt.Println("mapping   : not GitHub (gh-ctx.skip)")
		return nil
	}
	fmt.Printf("mapping   : %s on %s [via %s]\n", r.Account, r.Host, r.Via)

	if email := gitcfg.Email(r.Dir); email != "" {
		fmt.Printf("git email : %s\n", email)
	}

	active, err := ghcli.ActiveLogin()
	if err != nil {
		fmt.Printf("active gh : unknown (%v)\n", err)
		fmt.Println("hint      : this shell has no usable gh credentials; `direnv allow` this tree")
		return nil
	}
	mark := okMark(active == r.Account)
	fmt.Printf("active gh : %s %s\n", active, mark)
	if active != r.Account {
		fmt.Printf("hint      : active login %q != mapped %q — `direnv allow %s` or `cd` to re-trigger\n", active, r.Account, r.Dir)
	}
	return nil
}

// cmdInstallDirenv writes the use_gh_ctx function into direnvrc and a managed
// .envrc into each org root, never overwriting unmanaged files.
func cmdInstallDirenv(_ []string) error {
	incs, err := gitcfg.Includes()
	if err != nil {
		return err
	}
	if len(incs) == 0 {
		return fmt.Errorf("no includeIf gitdir rules found in git config; nothing to install")
	}
	home, _ := os.UserHomeDir()

	rcPath, changed, err := direnv.WriteRc()
	if err != nil {
		return fmt.Errorf("writing %s: %w", rcPath, err)
	}
	state := "unchanged"
	if changed {
		state = "updated"
	}
	fmt.Printf("direnvrc  : %s (%s)\n", rcPath, state)

	var toAllow []string
	var unmanaged []string
	for _, inc := range incs {
		base, ok := resolve.BaseDir(inc.GitDir, home)
		if !ok {
			fmt.Printf("skip      : %s (pattern not a plain directory prefix)\n", inc.GitDir)
			continue
		}
		if !inc.Configured() {
			fmt.Printf("skip      : %s (no [gh-ctx] account/skip in %s)\n", base, inc.Path)
			continue
		}
		if _, err := os.Stat(base); err != nil {
			fmt.Printf("skip      : %s (directory does not exist)\n", base)
			continue
		}
		body := direnv.EnvrcBody(inc.Account, inc.Host, inc.Skip)
		res, err := direnv.WriteEnvrc(base, body)
		if err != nil {
			return fmt.Errorf("writing %s/.envrc: %w", base, err)
		}
		fmt.Printf("envrc     : %s (%s)\n", res.Path, res.Status)
		switch res.Status {
		case direnv.Created, direnv.Updated:
			toAllow = append(toAllow, base)
		case direnv.Unmanaged:
			unmanaged = append(unmanaged, res.Path)
		}
	}

	if len(unmanaged) > 0 {
		fmt.Println("\nThese .envrc files already exist and were left untouched.")
		fmt.Println("Add this line to each yourself (then `direnv allow`):")
		for _, p := range unmanaged {
			fmt.Printf("  %s\n", p)
		}
	}
	if len(toAllow) > 0 {
		fmt.Println("\nTrust the generated files:")
		for _, d := range toAllow {
			fmt.Printf("  %s\n", direnv.AllowHint(d))
		}
	}
	return nil
}

// cmdDoctor verifies that the configuration is coherent and that every mapped
// account is actually authenticated with gh.
func cmdDoctor(_ []string) error {
	incs, err := gitcfg.Includes()
	if err != nil {
		return err
	}
	authed, err := ghcli.Accounts()
	if err != nil {
		return err
	}
	authedSet := map[string]bool{}
	for _, a := range authed {
		authedSet[a] = true
	}
	fmt.Printf("gh accounts authenticated: %v\n", authed)

	problems := 0
	wanted := map[string]bool{}
	for _, inc := range incs {
		switch {
		case inc.Skip:
			fmt.Printf("%s ok   skip (not GitHub)\n", okMark(true))
		case inc.Account == "":
			fmt.Printf("%s warn %s has no [gh-ctx] account or skip\n", okMark(false), inc.Path)
			problems++
		case !authedSet[inc.Account]:
			fmt.Printf("%s fail %s → account %q is not logged in (gh auth login --user %s)\n",
				okMark(false), inc.GitDir, inc.Account, inc.Account)
			problems++
		default:
			wanted[inc.Account] = true
			fmt.Printf("%s ok   %s → %s\n", okMark(true), inc.GitDir, inc.Account)
		}
	}

	// Authenticated accounts not referenced by any rule (informational).
	var orphan []string
	for _, a := range authed {
		if !wanted[a] {
			orphan = append(orphan, a)
		}
	}
	sort.Strings(orphan)
	for _, a := range orphan {
		fmt.Printf("%s note %s is authenticated but mapped to no directory\n", okMark(true), a)
	}

	if problems > 0 {
		return fmt.Errorf("%d problem(s) found", problems)
	}
	fmt.Println("all good")
	return nil
}

func okMark(ok bool) string {
	if ok {
		return "✓"
	}
	return "✗"
}
