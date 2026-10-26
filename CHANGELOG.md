# Changelog

All notable changes to TRUSS are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Dates below are when the work landed on `main`. No version has been tagged yet, so there is no released section.

## [Unreleased]

### Added

- Go module `github.com/afri-bit/truss` for Go 1.22.5, and the `truss` command (`cmd/truss`).
- Pipeline schema version `"1"`. A definition has `version`, `pipeline.name`, and one or more `steps`. Each step has a unique `id` and a single `run` string. The host shell interprets that string, including quotes, pipes, and environment expansion.
- `truss run <path>` loads the given file. Any filename is valid. `truss run` with no path loads `truss.yaml` from the working directory.
- Sequential host execution. Steps run in definition order, in the invoker's working directory, and with the invoker's environment.
- A streaming transcript. Each step prints its id, then that step's stdout and stderr as the process writes them. A successful run exits 0 and prints no status block.
- Fail-fast results. A non-zero exit, or a command that cannot be started, marks that step and the pipeline failed. Later steps stay unrun. The process exits 1 and prints one error line naming the pipeline, the step, and the exit code or start error.
- Load and validation errors before any command runs: unreadable path, invalid YAML, missing or empty pipeline name, missing or empty steps, missing or empty step id, duplicate step ids, missing or empty `run`, and a missing or unsupported schema version. Any other invocation prints `usage: truss run [<path>]` and exits 1.
- Structured run data for each executed step: status (`succeeded` or `failed`), exit code when a command ran, captured output, and failure context when a command did not start.
- MIT license.
- Specification-driven workflow for contributors: project constitution (1.1.0, including test-first development), product glossary in `docs/GLOSSARY.md`, and the host pipeline engine specification, plan, contracts, and tasks under `specs/001-host-pipeline-engine/`.

[Unreleased]: https://github.com/afri-bit/truss/commits/main
