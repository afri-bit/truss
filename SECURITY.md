# Security Policy

TRUSS runs user-defined commands on hosts and in containers, and its daemon
exposes a local API. Security reports are taken seriously.

## Supported Versions

TRUSS is pre-alpha and has no releases yet. Until 1.0.0, only the latest
release and the `main` branch receive security fixes.

| Version | Supported |
|---------|-----------|
| `main` | Yes |
| < latest release | No |

## Reporting a Vulnerability

**Do not open a public issue, discussion, or pull request.**

Report vulnerabilities privately through GitHub:
[**Report a vulnerability**](https://github.com/afri-bit/truss/security/advisories/new)
(repository **Security** tab → **Advisories** → **Report a vulnerability**).

Please include:

- the affected version or commit and your platform;
- a description of the issue and its impact;
- a minimal reproduction (pipeline YAML, commands, configuration);
- any known mitigation.

## What to Expect

- Acknowledgement within **5 business days**.
- An initial assessment within **10 business days**.
- Coordinated disclosure: we agree on a timeline with you, typically within
  90 days. You are credited in the advisory unless you prefer otherwise.

## Scope

In scope:

- the daemon API and Unix socket permissions;
- secret exposure in logs, events, errors, or run history;
- escaping the declared workspace or container boundaries;
- command injection through pipeline fields that should not be executed.

Out of scope: the commands a pipeline author deliberately runs. A pipeline is
code, so run pipelines only from sources you trust.
