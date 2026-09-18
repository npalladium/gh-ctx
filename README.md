# gh-ctx

`gh-ctx` selects a GitHub CLI account based on the current directory.

It reads account mappings from the Git configuration files referenced by your
`includeIf "gitdir:…"` rules. With [direnv](https://direnv.net), it exports
`GH_TOKEN` and `GH_HOST` separately in each shell instead of changing the
globally active [GitHub CLI](https://cli.github.com) account.

This is useful when work and personal repositories live under different
directory trees.

## Requirements

- Git
- [GitHub CLI](https://cli.github.com), with each account authenticated
- [direnv](https://direnv.net)

## Install

```sh
gh extension install npalladium/gh-ctx
```


## Configure

Suppose your global Git configuration separates work and personal repositories:

```ini
# ~/.config/git/config
[includeIf "gitdir:~/work/"]
    path = ~/.config/git/config.work

[includeIf "gitdir:~/personal/"]
    path = ~/.config/git/config.personal
```

Add the corresponding GitHub account to each included file:

```ini
# ~/.config/git/config.work
[gh-ctx]
    account = my-work-login

# ~/.config/git/config.personal
[gh-ctx]
    account = my-personal-login
```

The account value is the login shown by `gh auth status`.

Then install the direnv integration:

```sh
gh ctx install-direnv
```

This installs the `use_gh_ctx` direnv helper and creates a managed `.envrc` at
each supported directory root. Existing `.envrc` files not managed by `gh-ctx`
are left unchanged and reported; add `use gh_ctx <account> [host]` to those
files manually.

Trust newly created files:

```sh
direnv allow ~/work
direnv allow ~/personal
```

### Verify

```sh
cd ~/work
gh ctx which
gh ctx status
```

Open another shell under `~/personal` to verify that both shells can use their
configured accounts concurrently.

### Optional configuration

For GitHub Enterprise Server, specify a host:

```ini
[gh-ctx]
    account = my-work-login
    host = github.example.com
```

To mark a directory tree as intentionally unrelated to GitHub:

```ini
[gh-ctx]
    skip = true
```

The generated direnv configuration unsets `GH_TOKEN` and `GH_HOST` in that
tree.

### Manual shell integration

Without direnv, you can export credentials for the current directory manually:

```sh
eval "$(gh ctx env)"
```

In an unmapped directory, `gh ctx env` writes no shell commands and exits
successfully. It therefore does not clear credentials exported previously; use
direnv when credentials should unload automatically as you change directories.

## Commands

```text
gh ctx [which]          Show the account selected for the current directory
gh ctx env [DIR]        Print shell exports for a directory
gh ctx status           Show the selected account, active login, and Git email
gh ctx install-direnv   Install direnv integration for configured directory roots
gh ctx doctor           Check mappings and authenticated accounts
gh ctx version          Print the installed version
```

## Resolution

Inside a Git repository, `gh-ctx` asks Git for the effective `gh-ctx` settings,
allowing Git to evaluate the applicable `includeIf` rules.

Outside a repository, `gh-ctx` falls back to matching plain `gitdir` directory
prefixes from the global Git configuration. Complex glob and relative patterns
are not supported by this fallback and are skipped during direnv installation.

Tokens remain managed by GitHub CLI. `gh-ctx` retrieves them using
`gh auth token --user …` and does not read the operating-system credential
store directly.

## Development

```sh
just            # list tasks
just check      # gofmt + vet + test + goreleaser check
just snapshot   # build release artifacts without publishing
just install    # install this working copy as `gh ctx`
```

### Releasing

CI releases on pushed `v*` tags and can also be started with
`workflow_dispatch`. To release from a local checkout:

```sh
just release v1.0.0
```

## Acknowledgements

Inspired by [`automationpi/gh-context`](https://github.com/automationpi/gh-context),
a `kubectx`-style account switcher for GitHub CLI.

## License

[MIT](LICENSE)
