# 0027. Notifications are declared in the flow file

- Status: Proposed
- Date: 2026-11-27

## Context

The maintainer wants to hear when a scheduled flow fails (milestone v0.3.0, 2026-11-25), through
the desktop first. Constitution V says every process TRUSS starts is a step the user declared, and
secrets never reach logs.

## Decision

1. Notifications are declared per flow in a `notify` block: `desktop`, `webhook`, `run`. Declaring
   them is the user's consent to the processes and requests they cause: the system's notification
   tool, an HTTPS request, or the named command. Nothing is sent for a flow without the block.
2. Desktop notifications use the system's own tool, with no new dependency. Texts are passed as
   data (arguments, environment), never as script.
3. Webhook URLs are secrets in practice: `secret:NAME` reads them from the store, and no message
   ever quotes a URL. Plain `http` only to this machine.
4. Sending is best effort: bounded time, one retry, logged on failure, never a change to the run.
5. Email and a daemon-wide default wait; a `run` command covers anything else meanwhile.

## Consequences

- A notification needs one line in a flow file; a team channel needs one secret.
- On Linux, desktop notifications need `notify-send` and a desktop session.
