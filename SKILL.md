---
name: gh-stackx
description: Use the gh-stackx gh extension to submit, sync, and merge stacked pull requests when the GitHub Stacked PRs API is unavailable.
triggers:
  - user
  - model
allowed-tools:
  - read
  - bash
  - write
  - edit
  - grep
---

# gh-stackx

Use the `gh-stackx` `gh` extension to manage **stacked pull requests** on repositories that do not have the private-preview GitHub Stacked PRs API.

## When to use

- You want to open a series of dependent PRs.
- `gh stack submit` fails with `⚠ Stacked PRs are not enabled for this repository`.
- You need to keep PR bases correct as the stack is rebased or merged.
- You want to merge a stack from the top down without manually editing PR bases.

## Prerequisites

- `gh` CLI installed and authenticated.
- `github/gh-stack` extension installed for local stack operations:

  ```bash
  gh extension install github/gh-stack
  ```

- `gh-stackx` installed:

  ```bash
  gh extension install ThePlenkov/gh-stackx
  ```

## Core concepts

- **Trunk** — the base branch, usually `main`.
- **Stack** — an ordered list of branches where each branch builds on the previous one.
- **Bottom** — the branch closest to trunk.
- **Top** — the branch farthest from trunk.
- **Base** — the parent branch for a PR. The bottom layer uses trunk; every other layer uses the previous branch.

## Workflow

### Create a stack

```bash
git checkout main
git pull origin main

gh stackx init feature/auth
# work and commit

gh stackx add feature/api
# work and commit

gh stackx add feature/ui
# work and commit
```

### Submit PRs

```bash
gh stackx submit       # drafts
gh stackx submit --open
```

`submit` pushes all branches and creates/updates PRs bottom-up with the correct `--base` and `--head`.

### Sync after trunk or parent PR changes

```bash
gh stackx sync
gh stackx sync --remote upstream
```

`sync` runs `gh stack sync` and then updates every open PR base with `gh pr edit --base`.

### Merge top-down

```bash
gh stackx merge
gh stackx merge --squash
gh stackx merge --rebase
```

`merge` starts at the top of the stack and merges each open PR with `gh pr merge`.

## Local navigation

These pass through to `github/gh-stack`:

```bash
gh stackx view --json
gh stackx up
gh stackx down
gh stackx top
gh stackx bottom
gh stackx trunk
```

## Important rules

- Keep the working tree clean before `sync` or `merge`; rebase operations fail on uncommitted changes.
- Create PRs bottom-up so each base exists before the child PR is opened.
- Merge top-down so dependent PRs are merged before their parents move.
- Do not commit `.git/gh-stack`. It is local metadata.

## Troubleshooting

- `gh stackx view` fails: ensure `github/gh-stack` is installed.
- A PR has the wrong base: run `gh stackx sync`.
- `submit` or `merge` stops with errors: fix the underlying `gh pr` issue, then re-run.

## Further reading

- `docs/usage.md` — practical walkthrough with examples.
- `docs/methodology.md` — why and how stacked PRs work.
- `docs/spec.md` — full command and architecture specification.
- `docs/review.md` — review checklist for contributors.
- `AGENTS.md` — how to work on this repository.
