# Get notified

Hear when a run fails, recovers or ends, on your desktop, in a chat, or any way you like.

## On your desktop

```yaml
notify:
  desktop: true                     # on failure, the default
```

Linux needs `notify-send` (package `libnotify-bin` or `libnotify`); macOS shows it in the
Notification Center; Windows shows a toast.

## In Slack, Discord or Teams

Keep the webhook address in the secret store; it holds a token.

```console
$ truss secret set SLACK_WEBHOOK      # paste the https://hooks.slack.com/... address
```

```yaml
notify:
  on: [failure, recovered]
  webhook:
    url: secret:SLACK_WEBHOOK
    format: slack                     # slack, discord, teams, or generic (JSON)
```

## Anything else: a command

Send an email, a text message, or write to a log:

```yaml
notify:
  on: failure
  run: ./scripts/mail-me.sh
```

The command gets `TRUSS_NOTIFY_FLOW`, `TRUSS_NOTIFY_STATUS`, `TRUSS_NOTIFY_REASON`,
`TRUSS_NOTIFY_RUN_ID`, `TRUSS_NOTIFY_TRIGGER`, `TRUSS_NOTIFY_TITLE` and `TRUSS_NOTIFY_TEXT`.

## When

| `on` | You hear about |
|---|---|
| `failure` (default) | a failed run, or a scheduled time missed while the daemon was stopped |
| `success` | every successful run |
| `recovered` | the first success after a failure |
| `changed` | a run that ended differently from the one before |
| `always` | every run |

## Try it

```console
$ truss run backup --notify          # notify for this local run, as the daemon would
```

A notification that cannot be sent never changes the run. The daemon writes why to its log
(`truss daemon logs`); `truss run` prints it. See the
[flow file reference](pipeline-reference.md#notify-hear-how-a-run-ended) for every detail.
