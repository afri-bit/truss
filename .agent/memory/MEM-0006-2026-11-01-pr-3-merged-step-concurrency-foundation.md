---
id: MEM-0006
type: episodic
title: 2026-11-01 PR 3 merged: step concurrency foundation
summary: Spec 004 and the process-tree foundation landed; Windows CI needed three rounds to go green.
topics: [concurrency, windows, ci]
created: 2026-11-01T12:00:00Z
updated: 2026-11-01T12:00:00Z
author: orchestrator
source: PR #3
status: active
confidence: high
related: [specs/004-step-concurrency]
---

## What happened

PR #3 merged the spec 004 artifacts, per-Step process trees, a thread-safe run log, interrupt handling, and the CI changes.

## Outcome

Windows CI failed twice before passing: files were checked out with CRLF (gofmt rejects them), and a pid probe in a test was unreliable.

## Lesson

Cross-compiling and vetting is not enough for process code: only the real Windows runner showed the problems. Make tests wait on a signal the code itself produces, not on probing a process id. Add `* text=auto eol=lf` to any repository whose CI runs gofmt or bash on Windows.
