# gh-stackx

A `gh` CLI extension that wraps [`github/gh-stack`](https://github.com/github/gh-stack) and overrides
the remote operations that currently require the private-preview GitHub Stacked PRs API.

It is **not** a replacement for `gh stack` — it is a separate command `gh stackx` that delegates
to `github/gh-stack` for local operations and uses `gh pr create` / `gh pr edit` / `gh pr merge`
for the remote workflow.

## Why

`gh stack submit`, `gh stack link`, and `gh stack merge` depend on the GitHub Stack API, which is
not enabled for most repositories. This extension provides the same end-to-end workflow using
standard `gh pr` commands, so stacked PRs work without the private preview.

## Installation

1. Make sure `github/gh-stack` is installed:

   ```bash
   gh extension install github/gh-stack
   ```

2. Install this extension:

   ```bash
   gh extension install ThePlenkov/gh-stackx
   ```

The extension is a precompiled Go binary, so `gh` downloads the right executable for your platform (Windows, Linux, macOS, including ARM64).

## Usage

All local/navigation commands pass through to `github/gh-stack`:

```bash
gh stackx init
gh stackx add
gh stackx view
gh stackx up
gh stackx down
# ...
```

Remote operations are overridden:

### `gh stackx submit`

Pushes the stack and creates/updates PRs with `gh pr create` and `gh pr edit`.
New PRs are created as drafts unless `--open` is passed.

```bash
gh stackx submit --open
gh stackx submit --remote upstream
```

### `gh stackx sync`

Runs `gh stack sync`, then ensures every open PR has the correct base branch set via
`gh pr edit --base`.

```bash
gh stackx sync
```

### `gh stackx merge`

Merges the stack top-down with `gh pr merge`.

```bash
gh stackx merge
gh stackx merge --squash
gh stackx merge --rebase
```

## How it works

- `github/gh-stack` handles local stack metadata and commands (`init`, `add`, `view`, `rebase`, etc.).
- `gh-stackx` reads `gh stack view --json` to get the stack order and branches.
- For each branch, it calls `gh pr create`/`gh pr edit` with the correct `--base` and `--head`.
- `gh stackx merge` uses `gh pr merge` from the top of the stack down.

## Development

You need Go 1.23 or later to build from source:

```bash
cd gh-stackx
go build
gh extension install .
gh stackx --help
```

On Windows, `go build` produces `gh-stackx.exe`; on Linux and macOS it produces `gh-stackx`.

## License

MIT
