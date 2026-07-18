# Usage Guide

This is the practical, example-driven guide for using `gh-stackx`.

## Prerequisites

1. Install the `gh` CLI and authenticate (`gh auth login`).
2. Install the upstream `gh-stack` extension for local operations:

   ```bash
   gh extension install github/gh-stack
   ```

3. Install `gh-stackx`:

   ```bash
   gh extension install ThePlenkov/gh-stackx
   ```

Because `gh-stackx` is a precompiled Go binary, `gh` downloads the correct executable for your OS and architecture.

## Quickstart

Create a three-layer stack and open PRs for it:

```bash
# 1. Make sure you are on the latest trunk
git checkout main
git pull origin main

# 2. Create the first layer
gh stackx init feature/auth
# ... make changes and commit on feature/auth ...

# 3. Add the next layer
gh stackx add feature/api
# ... make changes and commit on feature/api ...

# 4. Add the third layer
gh stackx add feature/ui
# ... make changes and commit on feature/ui ...

# 5. Submit the stack as a set of PRs
gh stackx submit

# 6. Open a browser to review the bottom PR
gh pr view feature/auth
```

## Commands

### Local stack navigation

These pass through to `github/gh-stack`:

```bash
gh stackx init feature/auth    # start a stack from trunk
gh stackx add feature/api      # add a new layer on top
gh stackx view                 # show the stack and PR status
gh stackx view --json          # machine-readable stack status
gh stackx up                   # check out the next layer
gh stackx down                 # check out the previous layer
gh stackx top                  # jump to the top layer
gh stackx bottom               # jump to the bottom layer
gh stackx trunk                # jump to trunk
```

### Submit

Push all branches and create or update PRs with the correct `base` and `head`.

```bash
gh stackx submit          # create draft PRs, titles from commits
gh stackx submit --open   # create ready-for-review PRs
gh stackx submit --draft  # explicitly create drafts
```

`--open` and `--draft` are mutually exclusive. Without either, the default is draft.

### Sync

Rebase the stack onto the latest trunk and update every PR base branch.

```bash
gh stackx sync
gh stackx sync --remote upstream
```

Run this after the bottom PR is merged or after `main` changes.

### Merge

Merge the stack from the top down.

```bash
gh stackx merge
gh stackx merge --squash
gh stackx merge --rebase
gh stackx merge --no-delete-branch
```

`--squash` and `--rebase` are mutually exclusive. The default is a merge commit.

## Title and body generation

`gh stackx submit` generates PR titles from the first commit on each branch. If a branch has more than one commit, the first commit becomes the title and the remaining commit subjects become the body, one per line.

## Working with forks

`gh-stackx` resolves the push remote and the upstream parent repository from `gh repo view`. When the current repository is a fork, the bottom PR is created in the upstream parent repo; higher layers stay in the fork. Cross-host PR creation is not supported.

## Recovering from mistakes

- `gh stack sync` failed because of uncommitted changes? Commit or stash them and try again.
- A PR points to the wrong base? `gh stackx sync` updates bases automatically.
- A layer was merged manually? Run `gh stack view --json` to see the current state, then continue with `gh stackx submit` or `gh stackx merge`.
