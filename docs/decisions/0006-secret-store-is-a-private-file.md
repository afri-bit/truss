# 0006. The secret store is a private file in the TRUSS home

- Status: Accepted
- Date: 2026-11-12

## Context

A step could only get a credential from plaintext `env` in the pipeline file or from the daemon's
own environment. The constitution (principle V) forbids exposing secrets through logs and process
output handling. TRUSS needs a store the CLI and the daemon share, and a way to keep a printed
value out of every log.

## Decision

1. **A plain JSON file, `secrets.json`, in the TRUSS home**, mode `0600` on Linux and macOS,
   replaced atomically; a file readable by others is refused. Not encrypted.
2. **No OS keychain.** Using one means a third-party module (and cgo or a helper binary on some
   systems) or shelling out to `security`, `secret-tool` or PowerShell, which is implicit command
   execution and differs per OS. Constitution IV asks for the standard library unless a dependency
   is justified, and a new module needs the maintainer. A keychain backend stays possible behind
   the same `internal/secrets` API later; it would be a new ADR.
3. **Hidden prompt without a dependency.** `golang.org/x/term` would be a new module, so the
   prompt uses `ioctl` (Linux, macOS) and `SetConsoleMode` (Windows) from `syscall`. Other systems
   read the value from a pipe only.
4. **Redaction lives in the engine**, the one place every consumer's output passes (run log,
   daemon log, API, Desktop). A streaming matcher holds back only a tail that could still become a
   secret, so live output is not delayed otherwise.
5. **The store is read per run**, only when the pipeline declares secrets, and the engine receives
   it through `engine.WithSecrets`, a loader function, so `internal/engine` does not know the file.
6. **No daemon API for secrets**, so nothing can read a value back through HTTP or Desktop.
7. Secrets are global to the home; per-pipeline scoping is deferred.

## Consequences

- Good: no new module; one store for `truss run` and the daemon; missing secrets fail before any
  step; nothing in the API can leak a value.
- Bad: values are plaintext on disk. On Windows file permissions rely on the profile's ACL.
  Redaction is exact-match only (see the spec). Two simultaneous `secret set` calls can lose one
  update.
- Follow-ups: keychain backend; per-pipeline scope; a lock around store updates.
