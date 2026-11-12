# 0006. The daemon trusts its state files and registered paths only as far as it can check them

- Status: Accepted
- Date: 2026-11-12

## Context

An independent review of the daemon (PR #4) found that the home directory, the endpoint file
(`daemon.json`, which holds the token), registered pipeline files and the endpoint address were
trusted without checks. See `docs/reviews/2026-11-daemon-review.md`.

## Decision

1. **State files** are written to a new, uniquely named `0600` temporary file in the same
   directory, synced, then renamed over the target. The destination is never removed first, and a
   leftover or planted file at a predictable name is never reused or followed.
2. **The home directory** is made `0700` at every start on systems that honor modes. If that
   fails (for example another user owns it), `daemon start` fails. Windows file ACLs are not
   managed yet (follow-up); the docs say so.
3. **A client trusts `daemon.json` only if** the process at its address reports the recorded
   pid. A mismatch, a refused connection or a `401` mean "not running", and `daemon start` may
   then replace the file.
4. **Registered files** must be regular files of at most 1 MiB, at registration and at every
   reload. The daemon never blocks on a FIFO or reads without limit.
5. **Schedules** survive a manual trigger and a temporarily invalid file (spec 006 FR-021,
   FR-023).

## Consequences

- No wire change: the API, CLI output and run log are unchanged. `daemon start` can now fail on
  a home it does not own, and a 422 appears for non-regular or oversized files.
- The pid check does not stop a hostile local process from receiving the token sent in the
  status probe. Closing that needs a challenge-response or a ticket (follow-ups).

## Alternatives considered

- Ownership and mode checks on registered files (like ssh StrictModes): rejected for now; users
  would notice, and the maintainer should choose the opt-out. Follow-up.
