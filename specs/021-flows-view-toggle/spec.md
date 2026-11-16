# Feature Specification: Flows as cards or as a list

**Spec**: `021-flows-view-toggle` | **Created**: 2026-11-16 | **Status**: Implemented

Extends [`014-desktop-redesign`](../014-desktop-redesign/spec.md) (the Flows page).
User docs: [`docs/desktop.md`](../../docs/desktop.md). Risk class: tier 1 (visible behavior of
TRUSS Desktop; no API or file-format change). Scope set by the maintainer on 2026-11-16: the toggle
only. Grouping flows by project is a separate, later decision.

## User stories

1. **Scan many flows** (P1). With a dozen flows registered, cards take a lot of room. I switch the
   Flows page to a list and see every flow on one line, with its state, schedule and last run.
2. **Keep my choice** (P2). The page opens the way I left it, after a refresh and the next time I
   open Desktop in this browser.

## Requirements

- **FR-001** The Flows page has a switch with two choices, *Cards* and *List*, in the toolbar
  beside the filter. Cards is the default and is what the page showed before this spec. With no flow
  registered there is nothing to switch, and the page shows its empty state without the switch.
- **FR-002** *List* shows one table row per registered flow with these columns: state, flow (the
  name, a link to the flow's page, and its file path), schedule, next run, last run, duration, the
  shape of its step graph, and the actions. A flow whose file does not load shows its error in
  place of the schedule, next run, last run and duration, and *not runnable* as its shape.
- **FR-003** A row has the same actions as a card, with the same accessible names: *Run now*,
  *Pause schedule* or *Resume schedule* (disabled when the flow has no schedule), and *Unregister*
  (which asks first). They are icon buttons with a tooltip. Pressing a row outside its links
  and its actions cell opens the flow's page.
- **FR-004** Both views show the same flows, in the same order, and answer the same filter. The
  filter reads the text trimmed, against the name and the path together, whether it was just typed
  or the page was drawn again. Changing the view keeps the filter text and the half-typed register
  form, and does not change the count (*N of M*).
- **FR-005** The choice is kept in this browser with the other Desktop settings, and survives a
  reload. A missing or unknown stored value means *Cards*. The page keeps following the daemon
  while it shows the list (the same refresh as for cards).
- **FR-006** The switch is a group of two buttons; the one in use has `aria-pressed="true"`. It
  works with the keyboard, and keeps focus when pressed.

## Not in this spec

Grouping flows (by project or any other way), sorting by column, choosing which columns show, the
dashboard's lists, and any change to the daemon API.

## Compatibility

None. No daemon, CLI or API change. The only stored value is a new `flowsView` key in the browser's
existing `truss-settings` entry; an older Desktop ignores it.
