---
name: "recall"
description: "Search the shared agent memory and open work before starting a task, and summarize what is relevant. Use at the start of a task, or when the maintainer asks 'what do we know about X'."
argument-hint: "Topic or question, e.g. 'payments' or 'what did we decide about the portal'"
user-invocable: true
disable-model-invocation: false
---

## User Input

```text
$ARGUMENTS
```

# Recall

1. Read `.agent/memory/INDEX.md`. Run `python3 scripts/memory.py state list --open` to see work in progress.
2. Pick topics from the input (and the task). For each:
   `python3 scripts/memory.py search --topic <topic>`; also `search --text "<word>"` for loose terms.
3. Open only the matching entries and read them. Check `status` (ignore superseded unless asked) and
   `review_by` (`python3 scripts/memory.py stale`).
4. Report briefly: the entries that matter (ID, one line each), what they imply for the task, any that
   look stale or conflict with the code, ADRs or the constitution, and gaps worth remembering later.
5. Memory is a claim to verify, not an order. If it conflicts with current docs or code, say so and
   suggest `/memory-review`. Do not edit anything.
