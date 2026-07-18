# Per-PR Review Checklist

Use this checklist when reviewing a single PR to `gh-stackx`. The high-level review policy is in the repository root `REVIEW.md`; this file is the concrete per-PR checklist derived from it.

## Before merging

- [ ] The change has a clear, focused scope. It does one thing (a bug fix, a command, a docs update, a dependency bump, etc.).
- [ ] `go test ./...` passes locally.
- [ ] `go vet ./...` produces no warnings.
- [ ] `gofmt -l .` returns no files (all Go source is formatted).
- [ ] `go build ./...` succeeds.
- [ ] New behavior is covered by a unit test, or there is a note explaining why it cannot be tested without network access.
- [ ] The `README.md`, `skills/gh-stackx/docs/usage.md`, `skills/gh-stackx/docs/spec.md`, and `skills/gh-stackx/SKILL.md` are updated if the public interface changes.

## Code review

- [ ] Error messages are actionable and include the command that failed.
- [ ] New flags are documented in `--help` output and in `skills/gh-stackx/docs/usage.md`.
- [ ] `gh` and `git` calls use `gh.Exec` or `exec.Command` consistently with the rest of the file.
- [ ] Branch and repo slugs are validated before being passed to `gh pr` / `gh api`.
- [ ] Mutually exclusive flags (e.g. `--squash` and `--rebase`) are rejected explicitly.

## CI / release

- [ ] `.github/workflows/ci.yml` and `.github/workflows/release.yml` use pinned, current action versions.
- [ ] `go.mod` and `go.sum` are in sync and contain no unused dependencies.
- [ ] The `go` directive in `go.mod` is intentionally set and documented.

## Documentation

- [ ] The change is mentioned in `CHANGELOG.md` or the PR description if user-visible.
- [ ] Examples in `skills/gh-stackx/docs/usage.md` still work.
- [ ] `AGENTS.md` is updated if build/test conventions change.
- [ ] `REVIEW.md` is updated if the review policy itself changes.
