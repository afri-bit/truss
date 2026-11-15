# Independent review of the daemon, registry, runs and Desktop (PR #4)

Three read-only reviewers (security, portability, general) read the code; the author of the fixes
did not review them. Probe tests reproduced the items marked **proven**. Windows and macOS items
come from reading the code and need a real host.

## Fixed in this branch (each with a test that failed before)

| # | Rank | Finding | Failure scenario |
|---|---|---|---|
| 1 | High | A manual trigger resets the schedules (**proven**) | `daemon start`, then `trigger` before the first tick: the due catch-up or missed record is lost. |
| 2 | High | Invalid file at fire time records nothing (**proven**, FR-021) | Editing a file into an error before 09:00: no failed run appears; the schedule is gone. |
| 3 | Medium | `daemon.json` can be world-readable (**proven**) | A leftover `daemon.json.tmp` mode 0644 makes the token readable by other users. The temp name was also followed as a symlink. |
| 4 | Medium | Home directory not tightened | `TRUSS_HOME=/tmp/x` pre-created 0755 or by another user: they can swap `registry.json`. |
| 5 | Medium | FIFO or huge registered file hangs the daemon (**proven**) | Swap a registered file for a FIFO: ticks stop for good. `/dev/zero` exhausts memory. |
| 6 | Low-Med | `Dial` trusts any server at a stale address (**proven**) | After a crash, another program on the port blocks `daemon start` and sees the bearer token. |
| 7 | Low | `WriteJSON` removed the target before a retry (Windows) | A crash between remove and rename loses the registry, which then reads as empty. |
| 8 | Low | Run log open failure swallowed | Read-only run folder: the run succeeds with no log. Now `failed`, reason `open the run log`. |
| 9 | Low | UTF-8 split at a log chunk boundary; malformed `#` URL throws (Desktop) | Garbled characters in followed logs; blank page. |
| 10 | Low | No read or idle timeout on the API server | Idle keep-alive connections use up descriptors. (No test: server configuration.) |

## Follow-ups (not fixed here, with the reason)

| Rank | Finding | Why not now |
|---|---|---|
| High | Token is on the browser command line (`truss desktop`) | Changes how Desktop gets its token: a one-time ticket in the API. Maintainer decision. |
| High | Windows: token file inherits its folder's ACL | Needs `advapi32` calls or `golang.org/x/sys/windows` (a new dependency, maintainer approval), and a Windows host to test. |
| High | Windows: `DETACHED_PROCESS` shows a console window per step; `run` strings with double quotes are mangled by `cmd.exe` argument quoting | Execution contract change (tier 2); needs a Windows host and a spec. |
| High | FR-002: two daemons can serve one home (**proven**) | Needs a lock file (`flock` / `LockFileEx`) and its own ADR and portability review. |
| Medium | Steps outlive a hard-killed daemon (job object lacks `KILL_ON_JOB_CLOSE`; Unix needs a pgid record) | Platform work; needs Windows testing. |
| Medium | Windows: `SO_EXCLUSIVEADDRUSE`, UNC working directories, case-insensitive path equality | Windows host needed. |
| Medium | Refuse registered files or folders writable by others | Behavior users will notice; needs a decision on an opt-out. |
| Low | `save` errors ignored; cancel racing with success misreported; clock saved on wall-clock seconds; `since` applied to newly registered pipelines; executor keeps all step output in memory | Separate bugfixes. |
| Low | Challenge-response so the token is never sent to an unverified server | Pairs with the ticket work. |
| Low | CI stress job lacks `internal/runs` and `internal/daemon`; golangci-lint needs a Go 1.27 build | Workflow change for the maintainer. |
| Question | Should `unregister` cancel active runs? | The spec is silent; they keep running today. |
