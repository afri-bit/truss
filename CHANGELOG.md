# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
While the version is below 1.0.0, a breaking change to a public contract
(pipeline schema, CLI surface, or HTTP API `/v1`) increments the minor version.

## [Unreleased]

### Added

- Repository foundation: Go module layout with package documentation, a
  `truss` binary that reports build metadata, a Makefile, golangci-lint with
  architecture boundary rules, and CI on Linux, macOS, and Windows.
- Spec-driven development workflow with a status lifecycle, EARS acceptance
  criteria, templates, and a specs index.
- Architecture decision records 0001–0005.
- Draft specification 001: pipeline schema v1.
- Contributor, security, and code of conduct documents; agent instructions
  (`AGENTS.md`) with Claude Code skills and subagents.

### Changed

- License changed from MIT to Apache-2.0.

[Unreleased]: https://github.com/afri-bit/truss/commits/main
