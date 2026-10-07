# TRUSS

[![CI](https://github.com/afri-bit/truss/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/afri-bit/truss/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/afri-bit/truss.svg)](https://pkg.go.dev/github.com/afri-bit/truss)
[![Go version](https://img.shields.io/github/go-mod/go-version/afri-bit/truss)](go.mod)
[![License: Apache-2.0](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
![Status: pre-alpha](https://img.shields.io/badge/status-pre--alpha-orange.svg)

**Define a pipeline once. Run it anywhere.**

TRUSS is a provider-independent, portable CI pipeline runner written in Go.
You describe a pipeline in a small YAML file, and TRUSS runs it the same way
on your laptop, on Jenkins, or on GitHub Actions, across Linux, macOS, and
Windows. Each step declares whether it runs on the host or in a container.
`truss run` executes a pipeline directly in your terminal. The `trussd`
daemon runs pipelines on cron or manual triggers and keeps a searchable run
history. You stop debugging CI by pushing commits and waiting.

## Status

**Pre-alpha.** The repository foundation is in place, and the first
specification ([pipeline schema v1](specs/001-pipeline-schema-v1/spec.md)) is
in draft. Nothing is usable yet. Interfaces, the schema, and the CLI will
change until v0.1.0.

v0.1.0 scope: sequential steps, host and container executors, direct mode,
and a daemon with cron and manual triggers, SQLite run history, and explicit
workspaces. See the [specs index](specs/README.md) for progress.

## Quick start

> Placeholder. The commands below sketch the intended v0.1.0 experience and
> do not work yet. Command names and flags will be fixed by the CLI surface
> specification.

```sh
go install github.com/afri-bit/truss/cmd/truss@latest

truss init                      # scaffold pipeline.yml
truss run pipeline.yml          # run it now, in this terminal

truss daemon &                  # start trussd
truss bind pipeline.yml --workspace . --cron "0 2 * * *"
truss status                    # recent runs
truss logs --follow <run-id>    # stream a run
```

An illustrative pipeline mixing host and container steps is in
[`examples/pipeline.yml`](examples/pipeline.yml).

## Documentation

- [Vision and principles](docs/vision.md)
- [Architecture](docs/architecture.md): components, package layout, import
  rules, public contracts
- [Architecture decision records](docs/adr/README.md)
- [Specifications](specs/README.md)
- [User guide](docs/user/README.md) (grows with each release)
- [Development guide](docs/development/README.md): setup, testing, and the
  spec-driven workflow
- [Glossary](docs/GLOSSARY.md)

## Contributing

Contributions are welcome. TRUSS is built spec-first: read
[CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request. Coding
agents should start with [AGENTS.md](AGENTS.md). Please follow the
[Code of Conduct](CODE_OF_CONDUCT.md), and report vulnerabilities as
described in [SECURITY.md](SECURITY.md).

## License

[Apache License 2.0](LICENSE)
