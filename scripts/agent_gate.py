#!/usr/bin/env python3
"""Agent gate: checks that a pull request followed the pipeline in CLAUDE.md.

Python 3 standard library only. It runs in CI (`agent-gate` check) and locally:

    BASE_REF=develop python3 scripts/agent_gate.py

Inputs (environment variables):
    BASE_SHA or BASE_REF   what the branch is compared with (default: origin/develop, then develop)
    HEAD_SHA               the tip to check (default: HEAD)
    BRANCH                 branch name (default: the current branch)
    AUTHOR_LOGIN           who opened the PR; AGENT_LOGINS lists the agent accounts (comma separated)
    CHORE_TRUSTED          "1" when a maintainer with write access added the chore label
    PR_BODY                the PR description

Human pull requests only get the branch name and memory checks. An agent pull request (author in
AGENT_LOGINS, or a claude/ branch) also gets the pipeline checks. Exit 1 when a rule fails.
"""
from __future__ import annotations

import json
import os
import re
import subprocess
import sys
from pathlib import Path

BRANCH_RE = re.compile(r"^(feature|bugfix|hotfix)/[a-z0-9][a-z0-9._-]*$")
SPEC_BRANCH_RE = re.compile(r"^feature/(\d{3}-[a-z0-9-]+)$")
CODE_PATHS = (re.compile(r"^cmd/"), re.compile(r"^internal/"))
# Agents never change these; a human makes the change (listed at the top of the PR).
FORBIDDEN = (
    re.compile(r"^\.github/"),
    re.compile(r"^\.claude/(settings\.json|hooks/|agents/)"),
    re.compile(r"^scripts/"),
    re.compile(r"^\.golangci\.yml$"),
    re.compile(r"^go\.(mod|sum)$"),
    re.compile(r"^\.specify/memory/constitution\.md$"),
)
SPEC_FILES = ("spec.md", "plan.md", "tasks.md")
# A claude/ branch is named by the cloud runner, not by the constitution.
HARNESS_PREFIX = "claude/"


def git(*args: str, cwd: str | None = None) -> str:
    out = subprocess.run(["git", *args], cwd=cwd, capture_output=True, text=True)
    if out.returncode != 0:
        raise RuntimeError(f"git {' '.join(args)}: {out.stderr.strip()}")
    return out.stdout.strip()


def is_agent(branch: str, author: str, agents: list[str]) -> bool:
    return branch.startswith(HARNESS_PREFIX) or (author != "" and author in agents)


def work_id(branch: str) -> str:
    return branch.replace("/", "--")


def evaluate(root: str, base: str, head: str, branch: str, author: str, agents: list[str],
             chore_trusted: bool, body: str) -> list[str]:
    fails: list[str] = []
    files = [f for f in git("diff", "--name-only", f"{base}...{head}", cwd=root).splitlines() if f]
    agent = is_agent(branch, author, agents)

    # G1 branch name
    if not branch.startswith(HARNESS_PREFIX) and not BRANCH_RE.match(branch):
        fails.append(f"G1 branch '{branch}' must start with feature/, bugfix/ or hotfix/.")

    # G6 memory and state files are valid
    memory = Path(root, "scripts/memory.py")
    if memory.exists():
        r = subprocess.run([sys.executable, str(memory), "--root", root, "validate"],
                           capture_output=True, text=True)
        if r.returncode != 0:
            fails.append("G6 memory check failed: " + (r.stdout + r.stderr).strip().splitlines()[0])

    if not agent:
        return fails

    # G5 guardrail paths
    touched = [f for f in files if any(p.search(f) for p in FORBIDDEN)]
    if touched:
        fails.append("G5 an agent pull request must not change guardrail paths: " + ", ".join(touched[:8]))

    code = [f for f in files if any(p.search(f) for p in CODE_PATHS)]

    # G4 work state
    sid = work_id(branch)
    state_path = Path(root, ".agent/state", f"{sid}.json")
    state = None
    if not branch.startswith(HARNESS_PREFIX):
        if not state_path.exists():
            fails.append(f"G4 agent pull requests need a work state at .agent/state/{sid}.json.")
        else:
            try:
                state = json.loads(state_path.read_text())
            except ValueError:
                fails.append(f"G4 .agent/state/{sid}.json is not valid JSON.")
        if state and state.get("status") == "blocked" and "Needs a human" not in body:
            fails.append("G4 the work state is blocked; the description must start with 'Needs a human'.")

    # G2 spec artifacts and PR text for code changes
    if code and not chore_trusted:
        m = SPEC_BRANCH_RE.match(branch)
        specs = sorted(p.name for p in Path(root, "specs").glob("*") if p.is_dir())
        spec = m.group(1) if m and m.group(1) in specs else None
        if not spec:
            fails.append("G2 code changed but there is no matching specs/NNN-name for this branch "
                         "(a chore needs the maintainer's 'chore' label).")
        else:
            for name in SPEC_FILES:
                f = Path(root, "specs", spec, name)
                if not f.exists() or not f.read_text().strip():
                    fails.append(f"G2 specs/{spec}/{name} is missing or empty.")
            if f"specs/{spec}/" not in body:
                fails.append(f"G2 the description must link specs/{spec}/.")
            if not re.search(r"^Open tasks:\s*\S", body, re.M):
                fails.append("G2 the description needs a line 'Open tasks: T001, T002' or 'Open tasks: none'.")
            # G3 order: the first commit touching code already has the spec artifacts in its tree.
            for sha in git("rev-list", "--reverse", f"{base}..{head}", cwd=root).splitlines():
                changed = git("diff-tree", "--no-commit-id", "--name-only", "-r", sha, cwd=root).splitlines()
                if any(any(p.search(c) for p in CODE_PATHS) for c in changed):
                    missing = [n for n in SPEC_FILES
                               if subprocess.run(["git", "cat-file", "-e", f"{sha}^:specs/{spec}/{n}"],
                                                 cwd=root, capture_output=True).returncode != 0]
                    if missing:
                        fails.append(f"G3 commit {sha[:7]} changes code before specs/{spec}/ has "
                                     + ", ".join(missing) + ".")
                    break
    return fails


def main() -> int:
    root = git("rev-parse", "--show-toplevel")
    head = os.environ.get("HEAD_SHA") or "HEAD"
    base = os.environ.get("BASE_SHA") or os.environ.get("BASE_REF") or ""
    if not base:
        for cand in ("origin/develop", "develop"):
            if subprocess.run(["git", "rev-parse", "--verify", "-q", cand], cwd=root,
                              capture_output=True).returncode == 0:
                base = cand
                break
    if not base:
        print("agent-gate: cannot find a base; set BASE_SHA or BASE_REF", file=sys.stderr)
        return 2
    branch = os.environ.get("BRANCH") or git("branch", "--show-current", cwd=root)
    agents = [a.strip() for a in os.environ.get("AGENT_LOGINS", "claude[bot]").split(",") if a.strip()]
    fails = evaluate(root, base, head, branch, os.environ.get("AUTHOR_LOGIN", ""), agents,
                     os.environ.get("CHORE_TRUSTED") == "1", os.environ.get("PR_BODY", ""))
    if fails:
        print("agent-gate: FAILED")
        for f in fails:
            print(f"- {f}")
        return 1
    print("agent-gate: ok")
    return 0


if __name__ == "__main__":
    sys.exit(main())
