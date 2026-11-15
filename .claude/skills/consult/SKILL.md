---
name: "consult"
description: "Ask two agents the same question separately and in writing, compare their answers, and record the decision. Use for a public contract, execution-safety, dependency or boundary decision before deciding."
argument-hint: "The question, e.g. 'Should failed steps cancel running siblings?'"
user-invocable: true
disable-model-invocation: false
---

## User Input

```text
$ARGUMENTS
```

# Consult

A blind second opinion (constitution, Agent-to-agent protocol). The orchestrator decides; it does not
average opinions.

1. Write the question, the context (spec path, files), and 2 to 3 options in a short brief. If the
   input is empty, ask for the question in one sentence.
2. Pick two agents with different points of view: `planner` plus `security-reviewer` for safety,
   `planner` plus `reviewer` for quality, `architect` plus `portability-reviewer` for boundaries and
   OS behavior.
3. Spawn both **in parallel**, each with the same brief and the instruction to answer alone, in
   writing, with a recommendation and its main risk. Do not show either the other's answer.
4. Compare. If they agree, decide. If they differ, safety and data-loss concerns win; otherwise take
   the option that is simpler and stays inside the spec. If it is a product or scope question, stop:
   it belongs to the maintainer.
5. Record it: `python3 scripts/memory.py log --actor orchestrator --event decision --work <work-id>
   --summary "<question> -> <decision>" --refs <ADR or spec>`. A contract, boundary or dependency
   decision also needs `/adr`.
6. Report: question, each answer in one line, the decision, the reason. Do not commit.
