# Review Policy

These rules must always be observed when reviewing or contributing to `gh-stackx`. They are the source of truth for what makes a PR merge-ready.

## Non-negotiable quality gates

- `go test ./...` passes.
- `go vet ./...` produces no warnings.
- `gofmt -l .` returns no files (all Go source is formatted).
- `go build ./...` succeeds for all supported platforms (`linux`, `darwin`, `windows`; `amd64` and `arm64`).
- New behavior is covered by a unit test, or the PR description explains why it cannot be tested without network access.
- No secrets, credentials, or tokens are committed.
- No `Any`-typed or reflection-based lazy attribute access in Go without a documented reason.

## Dependencies and CI

- `go.mod` and `go.sum` must stay in sync (`go mod tidy`).
- The `go` directive in `go.mod` is set intentionally and matches the CI toolchain.
- CI workflow action versions are pinned to real, current release tags and verified against the GitHub API and the action's latest release page.
- The release workflow uses `generate_attestations: true` and a stable `go_version_file: go.mod`.

## Agent-facing artifacts

- The installable skill lives in `skills/gh-stackx/SKILL.md`. It must:
  - Have valid YAML frontmatter with `name`, `description`, `triggers`, and `allowed-tools`.
  - Be discoverable by `npx skills add . --list`.
  - Be installable by `npx skill skills/gh-stackx` when `SKILL_BASE_URL=https://github.com/ThePlenkov/gh-stackx/tree/main`.
- The root `REVIEW.md` (this file) is updated when the review policy changes.
- `AGENTS.md` is updated when build/test conventions or repository layout change.
- `skills/gh-stackx/docs/spec.md` is updated when command behavior, flags, or the data model change.
- `docs/usage.md` is updated when user-facing examples or workflows change.
- `README.md` is updated when quick-start steps or requirements change.

## Code and design

- The change has a clear, focused scope: one logical thing per PR (a bug fix, a command, a docs update, a dependency bump, etc.).
- Error messages are actionable and include the command that failed.
- New flags are documented in `--help` output and in `docs/usage.md`.
- `gh` and `git` calls use `gh.Exec` or `exec.Command` consistently with the rest of `main.go`.
- Branch and repo slugs are validated before being passed to `gh pr` / `gh api`.
- Mutually exclusive flags (e.g. `--squash` and `--rebase`) are rejected explicitly.
- Supply-chain safety: prefer dependency versions published at least seven days ago; avoid floating ranges (`latest`, `*`, unbounded `>=`).

## Merge readiness

A PR is merge-ready only when:

1. The scope and implementation match the PR title and description.
2. All local quality gates pass (`test`, `vet`, `gofmt`, `build`).
3. All required CI checks are green on the current HEAD.
4. All SAST/security annotations are triaged (fixed, suppressed with reason, or out-of-scoped with a linked issue).
5. All review threads are resolved with either a code change or a documented decision in the thread.
6. This `REVIEW.md` is respected; if the PR changes the policy, `REVIEW.md` itself is updated in the same PR.

For the practical per-PR checklist, see [`skills/gh-stackx/docs/review.md`](skills/gh-stackx/docs/review.md).
