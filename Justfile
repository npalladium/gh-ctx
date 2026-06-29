# gh-ctx tasks. Run `just` to list.
#
# Releases are normally cut by CI on a pushed tag, but `just release vX.Y.Z`
# performs the whole release from your machine (tag, push, build, upload) for
# when CI is unavailable.

# gh account that owns the repo / holds a token with `repo` scope.
release_account := "npalladium"

# goreleaser version constraint used locally and in CI.
goreleaser_version := "~> v2"

default:
    @just --list

# Format sources in place.
fmt:
    gofmt -w .

# Fail if any file is not gofmt-clean.
fmt-check:
    @out="$(gofmt -l .)"; if [ -n "$out" ]; then echo "needs gofmt:"; echo "$out"; exit 1; fi

vet:
    go vet ./...

test:
    go test -race ./...

# Build the extension binary into ./gh-ctx.
build:
    go build -o gh-ctx .

# Validate the goreleaser config (requires a git repo with a remote).
gr-check:
    goreleaser check

# Everything CI runs, locally.
check: fmt-check vet test gr-check

# Build release artifacts without publishing (sanity check the pipeline).
snapshot:
    goreleaser release --snapshot --clean --skip=publish
    @echo "artifacts in ./dist (asset names must end in <os>-<arch>):"
    @grep -oE '"name":"gh-ctx_[^"]*"' dist/artifacts.json | sort -u

# Install this working copy as a local gh extension for testing.
install: build
    gh extension remove ctx 2>/dev/null || true
    gh extension install .

# Cut a full release from your machine: tag -> push -> build -> upload.
# Usage: just release v1.2.3
release version:
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ ! "{{version}}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.]+)?$ ]]; then
        echo "error: version must look like v1.2.3 (got '{{version}}')" >&2
        exit 1
    fi
    if [ -n "$(git status --porcelain)" ]; then
        echo "error: working tree is dirty; commit or stash first" >&2
        exit 1
    fi
    echo "==> tests"
    go test -race ./...
    echo "==> goreleaser check"
    goreleaser check
    echo "==> tagging {{version}}"
    git tag -a "{{version}}" -m "Release {{version}}"
    git push origin "{{version}}"
    echo "==> releasing with goreleaser (account: {{release_account}})"
    GITHUB_TOKEN="$(gh auth token --user {{release_account}})" goreleaser release --clean
    echo "==> done: https://github.com/{{release_account}}/gh-ctx/releases/tag/{{version}}"

# Publish a release for a tag that already exists (no new tag created).
publish version:
    #!/usr/bin/env bash
    set -euo pipefail
    git rev-parse "{{version}}" >/dev/null 2>&1 || { echo "tag {{version}} does not exist" >&2; exit 1; }
    git checkout "{{version}}"
    GITHUB_TOKEN="$(gh auth token --user {{release_account}})" goreleaser release --clean

# Delete a botched release + tag (local and remote). Use with care.
unrelease version:
    #!/usr/bin/env bash
    set -euo pipefail
    gh release delete "{{version}}" --repo "{{release_account}}/gh-ctx" --yes --cleanup-tag || true
    git tag -d "{{version}}" 2>/dev/null || true
