---
name: release
description: Prepare a TRUSS release - promote CHANGELOG Unreleased entries, choose the semantic version, verify gates, and create the annotated tag. Use when the user asks to cut, prepare, tag, or publish a release or bump the version.
---

# Release

Versions follow [SemVer](https://semver.org/). Pre-1.0: a breaking change to
any public contract (pipeline schema, CLI surface, HTTP API `/v1`) bumps
MINOR; everything else bumps PATCH. The version lives only in the git tag
(`vX.Y.Z`) and is injected via ldflags; there is no version file.

## Checklist

1. **Branch state**: on an up-to-date `main`, clean working tree, CI green
   on the head commit.
2. **Scope check**: every spec targeted by this release is `done` in
   `specs/README.md`; no `implementing` spec is half-shipped behind
   user-visible behavior.
3. **Choose the version** from `CHANGELOG.md` Unreleased: `Removed` or a
   breaking `Changed` on a public contract → MINOR (pre-1.0) / MAJOR
   (post-1.0); `Added` → MINOR; only `Fixed`/`Security` → PATCH.
4. **Changelog**: move Unreleased entries under
   `## [X.Y.Z] - YYYY-MM-DD`, leave an empty `## [Unreleased]`, and update
   the compare links at the bottom.
5. **Contracts**: confirm each public-contract change has an ADR and that
   `docs/user/` reflects it (run `docs-writer` if unsure).
6. **Gates**: `make tidy fmt lint test test-race` and, with Docker,
   `make test-integration`. `git diff` must be empty after `tidy`/`fmt`.
7. **Build smoke test**: `make build VERSION=vX.Y.Z` then
   `./bin/truss` prints `truss vX.Y.Z`; cross-build check
   `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...` and the same for
   `darwin/arm64`.
8. **Commit** via PR: `chore(release): vX.Y.Z`.
9. **Tag** after merge, on the merge commit:
   `git tag -a vX.Y.Z -m "vX.Y.Z"` then `git push origin vX.Y.Z`.
   Never move or delete a pushed tag; fix forward with a new PATCH.
10. **Announce**: GitHub release notes = the changelog section.

Stop and ask before pushing a tag; it is irreversible in practice.
