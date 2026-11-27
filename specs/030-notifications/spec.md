# Feature Specification: Notifications

**Spec**: `030-notifications` | **Created**: 2026-11-27 | **Status**: Implemented

Extends [`006-daemon-registry-and-scheduling`](../006-daemon-registry-and-scheduling/spec.md) and
[`012-secrets`](../012-secrets/spec.md). Decision:
[ADR 0027](../../docs/decisions/0027-notifications-are-declared-in-the-flow-file.md). Risk class:
tier 2 (the flow file schema, processes TRUSS starts besides steps, outbound network, secrets).
Scope set by the maintainer on 2026-11-25 (milestone v0.3.0): "we have a cronjob and we forgot
but we got notified at least that run is being failed or successful"; through the desktop first;
email later.

## User stories

1. **A forgotten schedule fails** (P1). A nightly flow fails and I see a desktop notification.
2. **The team hears it** (P1). A failure posts to our Slack channel.
3. **Anything else** (P2). My own script sends an email.
4. **It works again** (P2). I hear when a failing flow recovers.

## Requirements

- **FR-001** A flow file may have `notify` with `on` (`failure`, `success`, `always`, `recovered`,
  `changed`; one or a list; default `failure`) and at least one of `desktop: true`, `webhook`
  (a URL or `{url, format}`) and `run` (a command).
- **FR-002** `failure` is a failed run or a scheduled time missed while the daemon did not run;
  `always` is any run that ended, except a skipped one that was not a missed schedule;
  `recovered` is a success after a failed previous run of the flow; `changed` is a status other than
  the previous run's. The previous run is the newest earlier run of the flow that was not skipped.
- **FR-003** Desktop: `notify-send` on Linux and the BSDs, `osascript` on macOS, a PowerShell toast
  on Windows. The title and text are passed as arguments or environment, never as code.
- **FR-004** Webhook URLs are `https://`, `http://` to this machine only, or `secret:NAME` (read from
  the secret store when sending; it must hold such a URL); no user name or password in the URL.
  Messages never quote a URL. Formats: `slack` and `teams` (`{"text"}`), `discord` (`{"content"}`),
  `generic` (the outcome as JSON). One retry after 2 seconds on a server error or no answer; none
  on a client error.
- **FR-005** `run` is a command for the host shell, started in the project root (the file's folder
  outside a project) with the daemon's environment and `TRUSS_NOTIFY_FLOW`, `_RUN_ID`, `_STATUS`,
  `_REASON`, `_TRIGGER`, `_TITLE`, `_TEXT`.
- **FR-006** Each channel gives up after 15 seconds. A notification that fails is written to the
  daemon log (`flow <name>: notification of run <id> not sent: ...`) and never changes the run.
- **FR-007** The daemon notifies for every run of a registered flow that ends, missed schedules
  included. `truss run --notify` notifies for a local run (run id `local`, trigger `local`, no
  previous run); without a notify block it says so.

## Not done

Email; daemon-wide default notifications for flows without a block; a tray icon (packaging).

## Compatibility

Additive: `notify` was an unknown field before.

## Acceptance

`internal/notify/notify_test.go` (when to notify, webhook formats and secret URLs, a client error
without the URL, the command's variables), `internal/pipeline/pattern_test.go` (`TestParse_Notify`),
`internal/daemon/notify_test.go` (a failure and a recovery notify, a plain success does not).
