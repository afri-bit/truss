# Git workflow

- One issue, one branch, one pull request. `develop` is the integration branch; `main` is
  releases. Never commit or push to either one.
- Branch names are `feature/<name>`, `bugfix/<name>` or `hotfix/<name>` (constitution, Branching).
  Lowercase, hyphenated, short. For a spec, `<name>` is the `specs/` folder name
  (`feature/003-step-dependencies`). Documentation, tests and maintenance use `feature/`. No other
  prefixes (`chore/`, `feat/`, `fix/`).
- Branch from `develop`. Open the PR against `develop`.
- Commit with `/git-commit`: one topic per commit, conventional subject, bullet body. Never add
  `Co-authored-by`, `Signed-off-by` or any tool attribution.
- Keep a PR under about 400 changed lines (excluding generated files and lockfiles) or say why.
- Unrelated fixes go in another branch; list them as follow-ups in the PR.
- Before opening or updating a PR run `gofmt -l .`, `go vet ./...`, `go test -race ./...`,
  `golangci-lint run ./...`, and the end-to-end script when the CLI changed (`/verify`).
- Open the PR yourself once checks and independent review pass. The maintainer reviews the PR,
  not the steps before it. Never merge, tag, or publish a release.
- Never skip hooks (`--no-verify`), force-push, rewrite pushed history or delete remote branches.
- Review is independent: the session that wrote the code never reviews it (`/review`).
- If the PR for your branch is already merged, start a new branch from the latest `develop`.
