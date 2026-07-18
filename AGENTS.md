# AGENTS.md

This file is for AI agents (and humans) working on `gh-stackx`.

## What this project is

`gh-stackx` is a `gh` CLI extension written in Go. It wraps the local stack management of `github/gh-stack` and replaces the remote operations (`submit`, `sync`, `merge`) with plain `gh pr create` / `gh pr edit` / `gh pr merge` calls. This makes stacked PRs work on repositories that do not have the private-preview GitHub Stacked PRs API.

## Build, test, lint

All commands are run from the repository root.

```bash
# Build the extension binary
go build ./...

# Run tests
go test ./...

# Run the vet and format gates used in CI
go vet ./...
test -z "$(gofmt -l .)"
```

## Project layout

- `main.go` — the entire extension. All commands and helpers live here.
- `main_test.go` — unit tests for pure helpers.
- `go.mod` / `go.sum` — Go module and dependencies.
- `.github/workflows/ci.yml` — tests, vet, and format checks on PRs and `main`.
- `.github/workflows/release.yml` — precompiled binary release when a `v*` tag is pushed.
- `skills/gh-stackx/` — installable agent skill for `gh-stackx`, including `SKILL.md` and `docs/`.
- `REVIEW.md` — review policy that must always be observed.

## Documentation layout

Two audiences, two trees:

**User-facing** (people who use `gh-stackx` as a tool):

- `README.md` — repository entry point and quickstart.
- `docs/` — end-user guides:
  - `docs/README.md` — doc index.
  - `docs/usage.md` — practical walkthrough.
  - `docs/methodology.md` — why and how stacked PRs work.

**Agent/contributor-facing** (people who modify this codebase):

- `AGENTS.md` (this file) — conventions for working on the repo.
- `REVIEW.md` — review policy.
- `skills/gh-stackx/SKILL.md` — installable agent skill (self-contained).
- `skills/gh-stackx/docs/spec.md` — implementation spec for code modifiers.
- `skills/gh-stackx/docs/review.md` — per-PR checklist for code modifiers.

When a public interface changes, update the four files listed in the per-PR checklist at [`skills/gh-stackx/docs/review.md`](skills/gh-stackx/docs/review.md).

## Conventions

- Keep the extension self-contained in `main.go`. Splitting into packages is only justified when the tool grows substantially.
- Prefer `gh.Exec` from `github.com/cli/go-gh/v2` for calling `gh`.
- Use `exec.Command` directly only for `git` plumbing calls.
- Table-driven tests for pure helpers. Integration tests that call `git` may create a temporary repository and change into it with `t.Chdir`.
- Do not commit secrets, tokens, or `.git/gh-stack` files.

## Common tasks

### Add a new flag to a subcommand

1. Parse it with `pflag` in the relevant `cmd*` function.
2. Pass it through to the `gh pr` / `gh stack` invocation.
3. Follow the documentation-update rule above (single source of truth is `skills/gh-stackx/docs/review.md`).
4. Add a test if the behavior is pure or can be exercised without network.

### Update dependencies

```bash
go get -u ./...
go mod tidy
```

### Update CI action versions

Pin to the latest stable release tag for each action and update the same tags in `skills/gh-stackx/docs/spec.md` if referenced.

### Release

Push a `v*` tag. The `release.yml` workflow builds cross-platform binaries with `cli/gh-extension-precompile`.

## Troubleshooting

- `gh stackx view` fails: ensure `github/gh-stack` is installed (`gh extension install github/gh-stack`).
- `gh extension install .` fails: build first with `go build`.
- Tests that call `git` fail locally: check that `git` is available and that a global commit template does not prepend unexpected text to commit subjects.
