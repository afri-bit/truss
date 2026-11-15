# Feature Specification: Secrets

**Spec**: `012-secrets` | **Created**: 2026-11-12 | **Status**: Implemented

**Input**: "There is no secret store: the only way to give a step a credential is plaintext `env`
in the pipeline file or the daemon's own environment."

Tier 2 (secrets handling, pipeline schema, CLI output). Decision record:
[ADR 0006](../../docs/decisions/0006-secret-store-is-a-private-file.md). User docs:
[`docs/pipeline-reference.md`](../../docs/pipeline-reference.md#secrets),
[`docs/cli.md`](../../docs/cli.md).

## User stories

1. **Keep a token out of the file** (P1). I store `NPM_TOKEN` once with `truss secret set`, name it
   in the pipeline, and the step sees it as an environment variable. The pipeline file stays
   safe to commit. *Test*: a step that checks `$NPM_TOKEN` succeeds only when it was stored.
2. **Fail early** (P1). A pipeline that needs a secret I never stored fails before any step
   starts and tells me which name to set. *Test*: no step output, exit 1, message names the secret.
3. **Not in the log** (P1). If a step prints a secret, the run log, the stored log, the API and
   Desktop show `***`. *Test*: values printed whole or split across output chunks never appear.
4. **Same store everywhere** (P1). `truss run` and the daemon read the same store.

## Requirements

### Store

- **FR-001** Secrets are kept in `secrets.json` in the TRUSS home (`~/.truss` or `TRUSS_HOME`),
  a JSON file (`{"version": 1, "secrets": {name: value}}`). On Linux and macOS the file is created with mode `0600`, is
  replaced atomically, and the home directory is `0700` when TRUSS creates it. A directory that group or others can write to is refused (they could swap the file), with an error that says to `chmod 700` it. Updates take a lock file next to the store, so two `truss secret` commands at once keep both changes. A file readable by group or others
  is refused with an error that names the file and says to `chmod 600` it; values are not read.
- **FR-002** On Windows, Unix mode bits do not exist. The file lives in the user's profile and
  relies on the ACL it inherits there. TRUSS does not change ACLs. This is documented, not hidden.
- **FR-003** Values are **not encrypted** at rest. Anyone who can read the file as your user, or
  a backup of your home directory, can read them.
- **FR-004** A name matches `[A-Za-z_][A-Za-z0-9_]*`, the same rule as `env` names. A value is
  any non-empty text. Values are never written anywhere but the store; names appear in `list`, `validate` and error messages.
- **FR-005** The store is global to the TRUSS home. Per-pipeline scoping is out of scope.

### Commands

- **FR-006** `truss secret set <name>` reads the value from standard input. When standard input
  is a terminal it prompts without echo and asks for the value twice; a mismatch stores nothing.
  When it is not a terminal (a pipe or file) the whole input is the value, minus one trailing
  line ending (`\n` or `\r\n`, not a lone `\r`). The value is never an argument. An empty value is an error. Output:
  `secret <name> saved`.
- **FR-007** `truss secret list` prints names only, sorted, one per line. It prints nothing for an
  empty store.
- **FR-008** `truss secret remove <name>` deletes a secret and prints `secret <name> removed`; an unknown name is an error.
- **FR-009** These commands do not need the daemon. The daemon reads the store at the start of
  each run, so a change applies to the next run without a restart.
- **FR-010** The daemon API has no secret endpoints. Secrets cannot be read or written through
  the API or Desktop.

### Pipeline file

- **FR-011** `secrets` is a list of names, allowed at pipeline, job and step level. Each is a
  valid name; a name listed twice in one list, a non-list or a non-string is a validation error
  that does not print any value. Layers combine like `env`: pipeline, then job, then step.
- **FR-012** Each declared secret becomes an environment variable of that name for the steps that
  declare it (directly or through their job or the pipeline) and for no other step. It overrides
  an `env` entry or inherited variable of the same name.
- **FR-013** If any declared secret is not in the store, the run fails before any step starts
  (no step is started, `truss run` exits 1, a daemon run is recorded as failed). The message
  lists the missing names and says `truss secret set <name>`; it never prints a value.
  A store that cannot be read fails the run the same way.
- **FR-014** A pipeline that declares no secret never reads the store.
- **FR-015** `truss validate` checks `secrets` syntax and lists the names a step uses
  (`step build uses secrets NPM_TOKEN`). It does not read the store and prints no values.

### Redaction

- **FR-016** In a run of a pipeline that declares secrets, every stored secret value (not only
  the declared ones) that appears in step stdout or stderr is replaced by `***` before it reaches
  the run log, `.truss/run.log`, the daemon's stored log, the API and Desktop, and before the
  in-memory step results. *Changed by [spec 016](../016-project-layout/spec.md): The file is `.truss/state/<flow>/run.log`.*
- **FR-017** A value split across several output chunks is still replaced. Stdout and stderr are matched separately, so output on one never releases a
  half-seen value on the other. Output is held back
  only while its tail could still turn out to be the start of a secret, and released when it
  cannot, or when the step ends. A value containing newlines is replaced as a whole.
- **FR-018** Redaction is exact text matching. It **cannot** catch a value that a step encodes
  (base64, hex, URL encoding), transforms (case, reversal, characters inserted), splits over
  several lines of its own,  or writes to a file, the network or anywhere other than stdout and
  stderr. A very short secret is replaced wherever it occurs, which can garble unrelated output.
- **FR-019** Secrets reach a step only through its environment. Anything the step itself does with
  them (`echo`, `env`, `set -x`) is the step's responsibility; the user must not print them.

## Compatibility

Additive. `secrets` is a new field, `truss secret` a new command, `secrets.json` a new file. Every
definition valid before keeps its meaning. No schema version bump. The Go API gains
`engine.WithSecrets`; `engine.Run` callers that pass no option behave as before.

## Out of scope

OS keychain integration, encryption at rest, per-pipeline scoping, secret rotation or expiry,
secret endpoints in the API, importing from `.env` files, masking values supplied by `env`.
