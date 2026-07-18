# Specification

This document describes the behavior of `gh-stackx` in enough detail that another agent or contributor can modify it safely.

## Purpose

`gh-stackx` is a `gh` CLI extension that enables stacked pull requests on repositories that do not have the GitHub Stacked PRs API. It delegates local stack management to `github/gh-stack` and implements the remote workflow (`submit`, `sync`, `merge`) with standard `gh pr` commands.

## Commands

| Command | Implemented by | Behavior |
| --- | --- | --- |
| `init`, `add`, `view`, `up`, `down`, `top`, `bottom`, `trunk`, `checkout` | `github/gh-stack` | Pass through unchanged. |
| `submit` | `gh-stackx` | Push the stack, then create or update PRs bottom-up with the correct `--base` and `--head`. Draft by default. |
| `sync` | `gh-stackx` | Run `gh stack sync`, then ensure every open PR has the correct base via `gh pr edit --base`. |
| `merge` | `gh-stackx` | Merge PRs from top to bottom using `gh pr merge`. |
| `version`, `--version`, `-v`, `view` (no args) | `github/gh-stack` | Pass through unchanged. |

## Data model

The extension reads stack state from `gh stack view --json` into the following Go structs:

```go
type Stack struct {
    Trunk     string   `json:"trunk"`
    Current   string   `json:"currentBranch"`
    Branches  []Branch `json:"branches"`
}

type Branch struct {
    Name        string `json:"name"`
    Head        string `json:"head"`
    Base        string `json:"base"`
    IsCurrent   bool   `json:"isCurrent"`
    IsMerged    bool   `json:"isMerged"`
    IsQueued    bool   `json:"isQueued"`
    NeedsRebase bool   `json:"needsRebase"`
}
```

A `Branch.Base` is considered during `sync` and `merge`; the actual PR base is computed by `baseForBranch`, which returns `stack.Trunk` for the bottom layer and the previous branch name for all others.

## Remote resolution

The extension resolves three pieces of information for every remote operation:

1. **Current repo**: from `gh repo view` (`nameWithOwner`, `url`, `parent`).
2. **Push remote / head repo**: from `git remote get-url --push <remote>`.
3. **Base repo for PRs**: for the bottom branch in a fork, the upstream parent repo; otherwise the current repo.

Remote URLs are parsed by `parseGitRemote`, which supports:

- `https://host/owner/repo.git`
- `git@host:owner/repo.git`
- `host:owner/repo.git`
- bracketed IPv6 hosts in scp-style URLs

The function rejects local paths and Windows drive-letter paths to avoid treating filesystem paths as GitHub remotes.

## PR creation rules

`submit` walks the stack bottom-up. For each branch it first tries to find an existing open PR.

- If an open PR exists and its base is wrong, update it with `gh pr edit --base`.
- If an open PR exists and is already ready, leave it.
- If no open PR exists, create one with `gh pr create` (or the REST API for organization-owned forks).

New PRs are created as drafts unless `--open` is passed.

## PR title and body

For each new PR, `submit` reads the commits between the base and the branch with `git log --format=%s --reverse`.

- If there is one commit, the title is the commit subject and the body is empty.
- If there are multiple commits, the title is the first subject and the body is the remaining subjects joined by `\n`.

## Merge rules

`merge` walks the stack top-down. For each branch it finds the open PR and runs `gh pr merge` with the chosen method (`--merge`, `--squash`, or `--rebase`). It optionally deletes branches and enables auto-merge.

## Error handling

Each subcommand collects per-branch errors and prints a summary. If any branch fails, the process exits with code `1` after attempting every branch.

## Dependencies

Direct dependencies:

- `github.com/cli/go-gh/v2` — `gh` API and command execution.
- `github.com/spf13/pflag` — POSIX-style flags.

Indirect dependencies:

- `github.com/cli/safeexec` — safe command execution.

## CI / CD

- `.github/workflows/ci.yml` runs on pushes and pull requests to `main`: format check, `go vet`, `go test`, and `go build`.
- `.github/workflows/release.yml` runs on `v*` tags and uses `cli/gh-extension-precompile` to build cross-platform binaries and publish a GitHub release.

## Versioning

Releases follow semantic versioning. The `release.yml` workflow is triggered by tags of the form `v*`.

## Security notes

- The extension does not persist credentials; it relies on `gh` authentication.
- It validates owner and repository slugs before using them in API calls.
- It rejects local filesystem paths and Windows drive letters as remotes.
