# gh-ctx

A [GitHub CLI](https://cli.github.com) extension that picks the **right `gh`
account per directory** — derived from your existing git
`includeIf "gitdir:…"` rules — and scopes the choice to **each shell** via
[direnv](https://direnv.net), instead of mutating global `gh` state.

If you already split work and personal git identities by directory, `gh-ctx`
makes `gh` follow the same boundary automatically, with **no parallel config to
keep in sync** and **no cross-terminal clobbering**.

## Why

Multi-account `gh` setups usually switch the *active* account with
`gh auth switch`. That writes one machine-global pointer, so two terminals in
two orgs fight over it — whoever switched last wins, and the other silently acts
as the wrong identity.

`gh-ctx` takes a different path:

- **Source of truth is your git config.** You already declare which directories
  are "work" vs "personal" via `includeIf "gitdir:…"`. `gh-ctx` reads those same
  rules; the only thing it adds is *which gh account* pairs with each — one
  `[gh-ctx] account = …` line per include file.
- **Selection is per-shell.** direnv exports `GH_TOKEN`/`GH_HOST` for the
  account that matches the current directory. `gh` honors those env vars over
  the global pointer, so every terminal is independently correct, forever. No
  global state is touched.

| | `gh auth switch` | `gh-ctx` |
| --- | --- | --- |
| Decision lives in | global `hosts.yml` | each shell's env |
| Concurrent terminals, different orgs | last switch wins | all independent |
| Config source | separate / manual | your git `includeIf` |

## Install

```sh
gh extension install npalladium/gh-ctx
```

Requires `git`, `direnv`, and `gh` (with each account already logged in via
`gh auth login`).

## Configure

### 1. Tell `gh-ctx` which gh account each tree uses

For every file your global git config pulls in with `includeIf "gitdir:…"`, add
a `[gh-ctx]` section. Given a typical setup:

```ini
# ~/.config/git/config
[includeIf "gitdir:~/work/"]
    path = ~/.config/git/config.work
[includeIf "gitdir:~/personal/"]
    path = ~/.config/git/config.personal
[includeIf "gitdir:~/gitlab/"]
    path = ~/.config/git/config.gitlab
```

add:

```ini
# ~/.config/git/config.work
[gh-ctx]
    account = my-work-login      # a `gh` account login

# ~/.config/git/config.personal
[gh-ctx]
    account = my-personal-login

# ~/.config/git/config.gitlab
[gh-ctx]
    skip = true                  # not GitHub: clear any inherited gh creds
```

Keys (all optional except one of `account`/`skip`):

| Key | Meaning |
| --- | --- |
| `account` | The `gh` account login to use in this tree. |
| `host` | GitHub host (default `github.com`); set for GHES. |
| `skip` | `true` marks a non-GitHub tree (e.g. GitLab); gh env is cleared there. |

Because resolution goes through git, **git decides** — all of git's `includeIf`
semantics (nested prefixes, `gitdir/i:`, etc.) apply, and your git identity and
gh account can never disagree.

### 2. Wire up direnv

```sh
gh ctx install-direnv
```

This:

- installs a `use_gh_ctx` helper into your `direnvrc`, and
- writes one managed `.envrc` at each org root (e.g. `~/work/.envrc` containing
  `use gh_ctx my-work-login`).

It is **idempotent** and **never overwrites an `.envrc` it doesn't manage** — if
you already have one, it prints the line to add yourself. Then trust each tree:

```sh
direnv allow ~/work
direnv allow ~/personal
```

Now `cd ~/work/anything` exports `my-work-login`'s token into that shell only; a
sibling terminal in `~/personal` uses the personal account at the same time.

## Commands

```
gh ctx [which]          Show the gh account resolved for the current directory
gh ctx env [DIR]        Print shell exports (GH_TOKEN/GH_HOST) for DIR
gh ctx status           Cross-check resolved account vs. active token vs. git email
gh ctx install-direnv   Write the direnv glue and one managed .envrc per org root
gh ctx doctor           Verify config and that mapped accounts are authenticated
gh ctx version
```

`gh ctx env` is `eval`-safe — in an unmapped directory it prints nothing and
exits 0 — so you can wire it up without direnv too:

```sh
eval "$(gh ctx env)"
```

## How resolution works

1. **Inside a repo:** `gh-ctx` asks git directly (`git config --get
   gh-ctx.account`), so git's own `includeIf` engine is authoritative.
2. **Outside a repo** (e.g. an org root during `install-direnv`): it matches the
   directory against the `includeIf "gitdir:…"` prefixes from your global config.

It never reads tokens from the OS keyring itself — it calls
`gh auth token --user …`, leaving secure storage entirely to `gh`.

## Development

```sh
just            # list tasks
just check      # gofmt + vet + test + goreleaser check
just snapshot   # build release artifacts without publishing
just install    # install this working copy as `gh ctx`
```

### Releasing

CI releases automatically on a pushed `v*` tag (see
`.github/workflows/release.yml`, runnable via **workflow_dispatch** too). To cut
a release entirely from your machine:

```sh
just release v1.0.0
```

This runs tests, tags, pushes the tag, and uses
[GoReleaser](https://goreleaser.com) to build the Linux/macOS (amd64/arm64)
binaries and upload them to a GitHub release. Assets are published as **raw
binaries** named `gh-ctx_<version>_<os>-<arch>`, which is what `gh`'s installer
expects (it matches assets by `<os>-<arch>` suffix and runs them directly).

## Acknowledgements

Inspired by [`automationpi/gh-context`](https://github.com/automationpi/gh-context),
a `kubectx`-style account switcher for `gh`. `gh-ctx` reworks that idea around
two different choices: deriving contexts from your existing git `includeIf`
rules instead of a separate context store, and scoping selection per-shell via
direnv instead of switching the global active account. Credit to that project
for the original inspiration.

## License

[MIT](LICENSE)
