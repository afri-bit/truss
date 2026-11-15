# 0005. Specifications record behavior; they do not gate code

- Status: Accepted
- Date: 2026-11-12

## Context

Constitution 1.3.0 required, for every behavioral change, the full Spec Kit sequence (specify,
clarify, plan, tasks, analyze) committed before any code, strict test-first for every task, and
an orchestrator that delegates all hands-on work. For the first four features this produced
careful contracts, and roughly eight documents per feature.

For the MVP the maintainer asked for the daemon, schedules, registry and desktop to be built
in one go, with specs used to document features without blocking implementation, and gave
permission to change the constitution and the hard rules.

## Decision

Constitution 2.0.0:

1. `spec.md` is required for behavior users can depend on, **in the same pull request**. It may
   be written before, during or after the code. Other Spec Kit artifacts are optional.
2. Tests ship with the behavior. Test-first is recommended, not enforced.
3. The main agent session may do the work itself. Delegation and the full pipeline remain the
   recommended route for contract and execution-safety changes.
4. The maintainer may widen an agent's autonomy for one session; the agent records it in the PR.

Unchanged: never merge, tag, force-push or push to `develop` or `main`; never weaken a test or
a check; independent review before merge; execution safety and portability rules.

## Consequences

- Large steps like the MVP are possible in one pull request.
- Less up-front scrutiny per change. The pull request review carries more weight, so each PR
  must say plainly what was not reviewed or verified.
- `docs/agents.md` and the `/feature` skill still describe the old mandatory flow and need a
  follow-up to match.
