---
id: MEM-0012
type: procedural
title: The Enter-to-register UI test flakes on a real route() race
summary: A forced repaint after Enter in the register form can be dropped, leaving the Flows list stale; do not retry or loosen the test.
topics: [desktop, testing, flaky-tests]
created: 2026-11-16T23:21:50Z
updated: 2026-11-16T23:21:50Z
author: orchestrator
source: feature/021-flows-view-toggle, diagnosed 2026-11-17 with a scratch copy of the test that dumped the API and the page
status: active
confidence: high
review_by: 2026-12-17
related: [specs/021-flows-view-toggle]
---

## What

The Desktop UI test "the picker walks up and home, and submitting with Enter registers once" (`tests/ui/pipelines.test.js`) fails about 1 run in 10 with a 30 s timeout waiting for the new flow's card. The daemon did register the flow (the API lists it and the form is cleared); the page never repaints.

Cause, in `route()` in `internal/webui/assets/app.js`: after Enter, `act()` calls `refresh(true)`, which is `route(true)`. If the 1.5 s timer's plain `route()` starts while that forced load is in flight, it bumps `generation`, so the forced repaint is dropped. The plain `route()` then sees focus in `#register-path` (counted as typing) and skips the repaint. The list stays stale until focus leaves the field. Clicking the Register button does not trigger it, because focus is then on a button.

## Why it matters

It is a product bug (a stale list after Enter), not only a test flake, and it can fail CI at random. Retrying until green or loosening the test hides it.

## How to apply

Fix it in its own bugfix branch: carry a pending force flag forward so a superseded forced route still paints in the route that wins. Keep the test as it is. Until then, a failure of that one test with this symptom is this race, not a regression of the change under review. Measured on 2026-11-17: 0 failures in 14 runs on `main`, about 5 in 45 on the 021 branch; samples are small and the branch did not change `route()`.
