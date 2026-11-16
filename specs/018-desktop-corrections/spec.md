# Feature Specification: Desktop corrections: chart, names and labels

**Spec**: `018-desktop-corrections` | **Created**: 2026-11-16 | **Status**: Implemented

Corrects [`014-desktop-redesign`](../014-desktop-redesign/spec.md) and
[`015-resource-metrics`](../015-resource-metrics/spec.md). User docs:
[`docs/desktop.md`](../../docs/desktop.md). Risk class: tier 1 (visible behavior of TRUSS Desktop;
no API or file-format change). Scope set by the maintainer on 2026-11-16. Written alongside the
implementation (constitution 2.0.0, principle I).

## User stories

1. **Read a run's resource chart** (P1). On a running run, both lines of the Resources chart stay
   inside the chart. Today the memory line can leave it and draw over the panel title.
2. **Find flows by that name** (P2). The sidebar item that lists the registered pipelines says
   *Flows*, the word the project folder (`.truss/flows/`) already uses.
3. **Re-run a run** (P2). The button that starts the same pipeline again says *Re-run*.

## Requirements

- **FR-001** On a run's Resources chart, every plotted value is drawn inside the plot area. CPU
  and memory share one axis (percent, from 0 to a ceiling set by the CPU peak). The memory line is
  scaled into that axis from its own peak, and a value outside the axis is drawn on its edge.
- **FR-002** The memory legend reads *mem, scaled*. The figures above the chart (CPU percent, RSS
  in MiB) are unchanged and stay the real numbers.
- **FR-003** The sidebar item, the page heading, the breadcrumbs and the *Jump to…* page entry
  and group for the list of registered pipelines say *Flows*. The addresses (`#/pipelines`) and the
  daemon API (`/pipelines`) do not change.
- **FR-004** The button on a finished run that requests another run of its pipeline is labeled
  *Re-run* (was *Run again*). It is still not offered for a run tracked from a shell
  ([spec 010](../010-run-tracking-and-queue/spec.md)).

## Out of scope

Renaming other uses of "pipeline" in Desktop text (the dashboard tile, the register form, empty
states), in the CLI, the API or the docs; a second axis for memory in MiB.

## Compatibility

Text and drawing only. No API, file format, address or CLI change.

## Verification

Browser tests in `tests/ui/`: the memory and CPU lines of a live run stay within the chart's
height (this failed before the change, with the memory line 348 units above it); the sidebar
link is named *Flows* and no *Pipelines* link remains; the run page's *Re-run* button starts a run.
Not verified: macOS and Windows (the browser tests run on Linux in CI; the chart test is skipped
where the daemon reports no resource figures).
