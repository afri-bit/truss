#!/usr/bin/env python3
"""Shared agent memory and work state for TRUSS.

Python 3 standard library only. No PyYAML, no network. See .agent/README.md for the formats.

    python3 scripts/memory.py validate
    python3 scripts/memory.py new --type semantic --title "..." --summary "..." --topics a,b
    python3 scripts/memory.py search --topic go
    python3 scripts/memory.py state init chore--example --title "..." --tier 0 --branch chore/example
    python3 scripts/memory.py log --actor planner --event decision --summary "..."

Set MEMORY_NOW (for example 2026-10-30T09:15:00Z) to fix the clock, and --root to point at
another repository root (used by the tests).
"""
from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import re
import sys
from pathlib import Path

MEMORY_TYPES = ("semantic", "episodic", "procedural")
MEMORY_STATUSES = ("active", "superseded", "archived")
CONFIDENCES = ("high", "medium", "low")
WORK_STATUSES = ("planned", "in_progress", "blocked", "in_review", "done", "abandoned")
CLOSED_STATUSES = ("done", "abandoned")
STALE_DAYS = 7  # open work not updated for this long is flagged at session start
EVENTS = (
    "work_started",
    "step_done",
    "decision",
    "blocker",
    "handoff",
    "memory_added",
    "memory_updated",
    "pr_opened",
    "note",
)
TIERS = (0, 1, 2)

REQUIRED_KEYS = (
    "id", "type", "title", "summary", "topics", "created", "updated",
    "author", "source", "status", "confidence",
)
OPTIONAL_KEYS = ("review_by", "supersedes", "related")
KEY_ORDER = REQUIRED_KEYS + OPTIONAL_KEYS
SECTIONS = {
    "semantic": ("What", "Why it matters", "How to apply"),
    "procedural": ("What", "Why it matters", "How to apply"),
    "episodic": ("What happened", "Outcome", "Lesson"),
}

STATE_KEYS = (
    "work_id", "title", "tier", "branch", "status", "step", "steps_done", "requirements",
    "specs", "owner_agent", "blockers", "open_questions", "next_actions", "pr",
    "created", "updated",
)
STATE_LISTS = ("steps_done", "requirements", "specs", "blockers", "open_questions", "next_actions")
EVENT_KEYS = ("ts", "actor", "event", "work_id", "summary", "refs")

TS_FORMAT = "%Y-%m-%dT%H:%M:%SZ"
TS_RE = re.compile(r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$")
DATE_RE = re.compile(r"^\d{4}-\d{2}-\d{2}$")
MEM_ID_RE = re.compile(r"^MEM-\d{4}$")
MEM_FILE_RE = re.compile(r"^(MEM-\d{4})-([a-z0-9]+(?:-[a-z0-9]+)*)\.md$")
TOPIC_RE = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
WORK_ID_RE = re.compile(r"^[a-z0-9][a-z0-9.-]*$")
RELATED_RES = [
    re.compile(p)
    for p in (
        r"^(BR|FR|NFR)-\d{3}$",
        r"^ADR-\d{4}$",
        r"^J-\d{3}$",
        r"^C-\d{3}$",
        r"^MEM-\d{4}$",
        r"^PR-\d+$",
        r"^specs/[A-Za-z0-9._/-]+$",
    )
]

# The memory is shared and committed, so it must never hold secrets or personal data.
HYGIENE = (
    ("private key", re.compile(r"-----BEGIN [A-Z ]*PRIVATE KEY-----")),
    ("AWS access key", re.compile(r"AKIA[0-9A-Z]{16}")),
    ("GitHub token", re.compile(r"ghp_[A-Za-z0-9]{36}|github_pat_[A-Za-z0-9_]{22,}")),
    ("Slack token", re.compile(r"xox[baprs]-[A-Za-z0-9-]{10,}")),
    ("Anthropic key", re.compile(r"sk-ant-[A-Za-z0-9_-]{20,}")),
    ("payment key", re.compile(r"[rs]k_live_[A-Za-z0-9]{16,}")),
    ("Google key", re.compile(r"AIza[0-9A-Za-z_-]{35}")),
    ("Midtrans key", re.compile(r"(SB-)?Mid-(server|client)-[A-Za-z0-9_-]{16,}")),
    ("Xendit key", re.compile(r"xnd_(development|production)_[A-Za-z0-9]{20,}")),
    ("email address", re.compile(r"[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}")),
    ("Indonesian phone number", re.compile(r"(?<![\w])(?:\+62|0)8\d{8,11}(?!\d)|\+62[\s-]?\d[\d\s-]{7,13}\d")),
    ("16-digit number (ID-like)", re.compile(r"(?<!\d)\d{16}(?!\d)")),
)


class Fail(Exception):
    """A user-facing error: printed without a traceback."""


# ---------------------------------------------------------------- time and paths

def parse_ts(value: str) -> dt.datetime:
    if not isinstance(value, str) or not TS_RE.match(value):
        raise ValueError(f"not an RFC 3339 UTC timestamp (YYYY-MM-DDTHH:MM:SSZ): {value!r}")
    return dt.datetime.strptime(value, TS_FORMAT)


def now() -> dt.datetime:
    fixed = os.environ.get("MEMORY_NOW")
    if fixed:
        try:
            return parse_ts(fixed)
        except ValueError as exc:
            raise Fail(f"MEMORY_NOW: {exc}")
    return dt.datetime.now(dt.timezone.utc).replace(tzinfo=None, microsecond=0)


def fmt_ts(value: dt.datetime) -> str:
    return value.strftime(TS_FORMAT)


class Layout:
    def __init__(self, root: Path):
        self.root = root
        self.base = root / ".agent"
        self.memory = self.base / "memory"
        self.state = self.base / "state"
        self.log = self.base / "log"
        self.index = self.memory / "INDEX.md"

    def rel(self, path: Path) -> str:
        try:
            return str(path.relative_to(self.root))
        except ValueError:
            return str(path)


# ---------------------------------------------------------------- front matter

def split_front_matter(text: str) -> tuple[list[str], str]:
    lines = text.splitlines()
    if not lines or lines[0].strip() != "---":
        raise Fail("missing front matter (the file must start with ---)")
    for i in range(1, len(lines)):
        if lines[i].strip() == "---":
            return lines[1:i], "\n".join(lines[i + 1:]).strip("\n")
    raise Fail("front matter is not closed with ---")


def parse_value(raw: str):
    raw = raw.strip()
    if raw.startswith("[") and raw.endswith("]"):
        inner = raw[1:-1].strip()
        if not inner:
            return []
        return [part.strip().strip("\"'") for part in inner.split(",")]
    if len(raw) >= 2 and raw[0] == raw[-1] and raw[0] in "\"'":
        return raw[1:-1]
    return raw


def parse_entry(text: str) -> tuple[dict, str]:
    lines, body = split_front_matter(text)
    meta: dict = {}
    for line in lines:
        if not line.strip():
            continue
        match = re.match(r"^([a-z_]+):\s*(.*)$", line)
        if not match:
            raise Fail(f"bad front matter line: {line!r}")
        key, raw = match.groups()
        if key in meta:
            raise Fail(f"duplicate front matter key: {key}")
        meta[key] = parse_value(raw)
    return meta, body


def render_value(value) -> str:
    if isinstance(value, list):
        return "[" + ", ".join(value) + "]"
    return str(value)


def render_entry(meta: dict, body: str) -> str:
    out = ["---"]
    for key in KEY_ORDER:
        if key not in meta:
            continue
        value = meta[key]
        if key == "related" and not value:
            continue
        out.append(f"{key}: {render_value(value)}")
    out.append("---")
    out.append("")
    out.append(body.strip("\n"))
    return "\n".join(out) + "\n"


# ---------------------------------------------------------------- hygiene

def hygiene_problems(text: str) -> list[str]:
    return [name for name, pattern in HYGIENE if pattern.search(text)]


# ---------------------------------------------------------------- memory entries

def slugify(title: str, limit: int = 50) -> str:
    slug = re.sub(r"[^a-z0-9]+", "-", title.lower()).strip("-")
    if len(slug) > limit:
        slug = slug[:limit].rsplit("-", 1)[0] or slug[:limit]
    if not slug:
        raise Fail("the title must contain letters or digits")
    return slug


def check_meta(meta: dict) -> list[str]:
    errs: list[str] = []
    for key in meta:
        if key not in KEY_ORDER:
            errs.append(f"unknown front matter key: {key}")
    for key in REQUIRED_KEYS:
        if key not in meta:
            errs.append(f"missing required key: {key}")
    if errs:
        return errs

    def need_str(key: str, limit: int | None = None) -> None:
        value = meta[key]
        if not isinstance(value, str) or not value.strip():
            errs.append(f"{key} must be a non-empty string")
        elif limit and len(value) > limit:
            errs.append(f"{key} is longer than {limit} characters")

    for key in ("title", "summary", "author", "source"):
        need_str(key, 100 if key == "title" else 240 if key == "summary" else None)
    if not MEM_ID_RE.match(str(meta["id"])):
        errs.append(f"id must look like MEM-0001, got {meta['id']!r}")
    if meta["type"] not in MEMORY_TYPES:
        errs.append(f"type must be one of {', '.join(MEMORY_TYPES)}, got {meta['type']!r}")
    if meta["status"] not in MEMORY_STATUSES:
        errs.append(f"status must be one of {', '.join(MEMORY_STATUSES)}, got {meta['status']!r}")
    if meta["confidence"] not in CONFIDENCES:
        errs.append(f"confidence must be one of {', '.join(CONFIDENCES)}, got {meta['confidence']!r}")

    topics = meta["topics"]
    if not isinstance(topics, list) or not topics:
        errs.append("topics must be a non-empty list like [rust, owner-preferences]")
    else:
        for topic in topics:
            if not TOPIC_RE.match(topic):
                errs.append(f"topic must be lowercase kebab-case, got {topic!r}")

    stamps = {}
    for key in ("created", "updated"):
        try:
            stamps[key] = parse_ts(meta[key])
        except ValueError as exc:
            errs.append(f"{key}: {exc}")
    if len(stamps) == 2 and stamps["updated"] < stamps["created"]:
        errs.append("updated is earlier than created")

    if "review_by" in meta and not (
        isinstance(meta["review_by"], str) and DATE_RE.match(meta["review_by"]) and _valid_date(meta["review_by"])
    ):
        errs.append(f"review_by must be a date YYYY-MM-DD, got {meta['review_by']!r}")
    if "supersedes" in meta and not MEM_ID_RE.match(str(meta["supersedes"])):
        errs.append(f"supersedes must look like MEM-0001, got {meta['supersedes']!r}")
    if "related" in meta:
        related = meta["related"]
        if not isinstance(related, list):
            errs.append("related must be a list like [FR-012, ADR-0002]")
        else:
            for ref in related:
                if not any(r.match(ref) for r in RELATED_RES):
                    errs.append(
                        f"related has an unknown reference {ref!r} "
                        "(use BR-/FR-/NFR-nnn, ADR-nnnn, J-nnn, C-nnn, MEM-nnnn, PR-n or specs/...)"
                    )
    return errs


def _valid_date(value: str) -> bool:
    try:
        dt.datetime.strptime(value, "%Y-%m-%d")
        return True
    except ValueError:
        return False


def check_body(meta: dict, body: str) -> list[str]:
    errs: list[str] = []
    headings = re.findall(r"^##\s+(.+?)\s*$", body, flags=re.M)
    for section in SECTIONS.get(meta.get("type"), ()):
        if section not in headings:
            errs.append(f"missing section '## {section}'")
    if "TODO:" in body:
        errs.append("the body still has a TODO: placeholder")
    return errs


def load_entries(layout: Layout) -> list[dict]:
    """Strictly load every memory entry. Raises Fail on the first unreadable file."""
    entries = []
    if not layout.memory.is_dir():
        return entries
    for path in sorted(layout.memory.glob("MEM-*.md")):
        try:
            meta, body = parse_entry(path.read_text(encoding="utf-8"))
        except Fail as exc:
            raise Fail(f"{layout.rel(path)}: {exc}")
        entries.append({"path": path, "meta": meta, "body": body})
    return entries


def render_index(entries: list[dict]) -> str:
    rows = sorted(entries, key=lambda e: e["meta"]["id"])
    active = sum(1 for e in rows if e["meta"]["status"] == "active")
    out = [
        "# Memory index",
        "",
        "<!-- GENERATED by `python3 scripts/memory.py index`. Do not edit by hand. -->",
        "",
        f"{len(rows)} entries, {active} active. Search with `python3 scripts/memory.py search --topic <topic>`.",
        "",
        "| ID | Type | Title | Topics | Updated | Status | Confidence |",
        "|---|---|---|---|---|---|---|",
    ]
    for e in rows:
        m = e["meta"]
        title = str(m["title"]).replace("|", "\\|")
        topics = ", ".join(m["topics"])
        out.append(
            f"| [{m['id']}]({e['path'].name}) | {m['type']} | {title} | {topics} "
            f"| {str(m['updated'])[:10]} | {m['status']} | {m['confidence']} |"
        )
    return "\n".join(out) + "\n"


def write_index(layout: Layout) -> None:
    layout.memory.mkdir(parents=True, exist_ok=True)
    layout.index.write_text(render_index(load_entries(layout)), encoding="utf-8")


# ---------------------------------------------------------------- work state

def new_work_id(branch: str) -> str:
    return re.sub(r"[^a-z0-9.-]+", "-", branch.lower().replace("/", "--")).strip("-")


def check_state(state: dict, expected_id: str | None = None) -> list[str]:
    errs: list[str] = []
    for key in state:
        if key not in STATE_KEYS:
            errs.append(f"unknown key: {key}")
    for key in STATE_KEYS:
        if key not in state:
            errs.append(f"missing key: {key}")
    if errs:
        return errs
    if not isinstance(state["work_id"], str) or not WORK_ID_RE.match(state["work_id"]):
        errs.append(f"work_id must be lowercase letters, digits, '.' and '-', got {state['work_id']!r}")
    elif expected_id and state["work_id"] != expected_id:
        errs.append(f"work_id {state['work_id']!r} does not match the file name {expected_id!r}")
    for key in ("title", "branch", "step", "owner_agent"):
        if not isinstance(state[key], str):
            errs.append(f"{key} must be a string")
    if not state["title"]:
        errs.append("title must not be empty")
    if isinstance(state["tier"], bool) or state["tier"] not in TIERS:
        errs.append(f"tier must be one of {', '.join(map(str, TIERS))}, got {state['tier']!r}")
    if state["status"] not in WORK_STATUSES:
        errs.append(f"status must be one of {', '.join(WORK_STATUSES)}, got {state['status']!r}")
    for key in STATE_LISTS:
        value = state[key]
        if not isinstance(value, list) or not all(isinstance(v, str) for v in value):
            errs.append(f"{key} must be a list of strings")
    if state["pr"] is not None and not isinstance(state["pr"], str):
        errs.append("pr must be a URL string or null")
    for key in ("created", "updated"):
        try:
            parse_ts(state[key])
        except ValueError as exc:
            errs.append(f"{key}: {exc}")
    return errs


def state_path(layout: Layout, work_id: str) -> Path:
    if not WORK_ID_RE.match(work_id):
        raise Fail(f"bad work id {work_id!r}: use lowercase letters, digits, '.' and '-' "
                   "(the branch name with '/' replaced by '--')")
    return layout.state / f"{work_id}.json"


def load_state(layout: Layout, work_id: str) -> dict:
    path = state_path(layout, work_id)
    if not path.is_file():
        raise Fail(f"no work state for {work_id!r} (run: state init {work_id} ...)")
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        raise Fail(f"{layout.rel(path)}: invalid JSON: {exc}")


def save_state(layout: Layout, state: dict) -> None:
    errs = check_state(state)
    if errs:
        raise Fail("invalid work state: " + "; ".join(errs))
    layout.state.mkdir(parents=True, exist_ok=True)
    ordered = {key: state[key] for key in STATE_KEYS}
    state_path(layout, state["work_id"]).write_text(
        json.dumps(ordered, indent=2, ensure_ascii=False) + "\n", encoding="utf-8"
    )


def all_states(layout: Layout) -> list[dict]:
    states = []
    if layout.state.is_dir():
        for path in sorted(layout.state.glob("*.json")):
            try:
                states.append(json.loads(path.read_text(encoding="utf-8")))
            except json.JSONDecodeError as exc:
                raise Fail(f"{layout.rel(path)}: invalid JSON: {exc}")
    return states


# ---------------------------------------------------------------- event log

def check_event(event: dict, month: str | None = None) -> list[str]:
    errs: list[str] = []
    if not isinstance(event, dict):
        return ["each line must be a JSON object"]
    for key in event:
        if key not in EVENT_KEYS:
            errs.append(f"unknown key: {key}")
    for key in EVENT_KEYS:
        if key not in event:
            errs.append(f"missing key: {key}")
    if errs:
        return errs
    try:
        stamp = parse_ts(event["ts"])
        if month and stamp.strftime("%Y-%m") != month:
            errs.append(f"ts {event['ts']} is not in the month of the file ({month})")
    except ValueError as exc:
        errs.append(f"ts: {exc}")
    for key in ("actor", "summary"):
        if not isinstance(event[key], str) or not event[key].strip():
            errs.append(f"{key} must be a non-empty string")
    if event["event"] not in EVENTS:
        errs.append(f"event must be one of {', '.join(EVENTS)}, got {event['event']!r}")
    if event["work_id"] is not None and not (
        isinstance(event["work_id"], str) and WORK_ID_RE.match(event["work_id"])
    ):
        errs.append("work_id must be a work id string or null")
    if not isinstance(event["refs"], list) or not all(isinstance(r, str) for r in event["refs"]):
        errs.append("refs must be a list of strings")
    return errs


def append_event(layout: Layout, actor: str, event: str, work_id: str | None, summary: str,
                 refs: list[str]) -> dict:
    stamp = now()
    record = {
        "ts": fmt_ts(stamp), "actor": actor, "event": event,
        "work_id": work_id, "summary": summary, "refs": refs,
    }
    errs = check_event(record)
    if errs:
        raise Fail("invalid event: " + "; ".join(errs))
    layout.log.mkdir(parents=True, exist_ok=True)
    path = layout.log / f"{stamp.strftime('%Y-%m')}.jsonl"
    with path.open("a", encoding="utf-8") as handle:
        handle.write(json.dumps(record, ensure_ascii=False) + "\n")
    return record


# ---------------------------------------------------------------- validate

def validate_all(layout: Layout) -> tuple[list[str], dict]:
    errs: list[str] = []
    stats = {"entries": 0, "work": 0, "events": 0, "stale": 0}

    def err(path: Path, message: str) -> None:
        errs.append(f"{layout.rel(path)}: {message}")

    parsed: list[dict] = []
    if layout.memory.is_dir():
        for path in sorted(layout.memory.iterdir()):
            if path.name == "INDEX.md" or path.name.startswith("."):
                continue
            if not MEM_FILE_RE.match(path.name):
                err(path, "unexpected file; entries are named MEM-nnnn-slug.md")
        for path in sorted(layout.memory.glob("MEM-*.md")):
            text = path.read_text(encoding="utf-8")
            file_match = MEM_FILE_RE.match(path.name)
            try:
                meta, body = parse_entry(text)
            except Fail as exc:
                err(path, str(exc))
                continue
            problems = check_meta(meta)
            if not problems:
                problems += check_body(meta, body)
                if file_match and file_match.group(1) != meta["id"]:
                    problems.append(f"id {meta['id']} does not match the file name")
            for problem in problems:
                err(path, problem)
            for name in hygiene_problems(text):
                err(path, f"contains a {name}; memory must hold no secrets or personal data")
            if not problems:
                parsed.append({"path": path, "meta": meta, "body": body})

    ids: dict[str, Path] = {}
    for e in parsed:
        mid = e["meta"]["id"]
        if mid in ids:
            err(e["path"], f"duplicate id {mid} (also in {ids[mid].name})")
        ids[mid] = e["path"]
    by_id = {e["meta"]["id"]: e for e in parsed}
    superseded_by: dict[str, str] = {}
    for e in parsed:
        target = e["meta"].get("supersedes")
        if target:
            if target not in by_id:
                err(e["path"], f"supersedes {target}, which does not exist")
            elif by_id[target]["meta"]["status"] != "superseded":
                err(e["path"], f"supersedes {target}, but {target} is not marked status: superseded")
            else:
                superseded_by[target] = e["meta"]["id"]
        for ref in e["meta"].get("related", []):
            if MEM_ID_RE.match(ref) and ref not in by_id and len(parsed) == len(list(layout.memory.glob("MEM-*.md"))):
                err(e["path"], f"related {ref} does not exist")
    for e in parsed:
        mid, status = e["meta"]["id"], e["meta"]["status"]
        if status == "superseded" and mid not in superseded_by:
            err(e["path"], "status is superseded but no entry has supersedes: " + mid)

    stats["entries"] = len(parsed)
    today = now().strftime("%Y-%m-%d")
    stats["stale"] = sum(
        1 for e in parsed if e["meta"]["status"] == "active" and e["meta"].get("review_by", "9999-12-31") < today
    )

    if layout.memory.is_dir() and len(parsed) == len(list(layout.memory.glob("MEM-*.md"))):
        expected = render_index(parsed)
        actual = layout.index.read_text(encoding="utf-8") if layout.index.is_file() else None
        if actual != expected:
            err(layout.index, "missing or out of date; run: python3 scripts/memory.py index")

    if layout.state.is_dir():
        for path in sorted(layout.state.iterdir()):
            if path.suffix != ".json":
                err(path, "unexpected file; work states are <work-id>.json")
                continue
            text = path.read_text(encoding="utf-8")
            try:
                state = json.loads(text)
            except json.JSONDecodeError as exc:
                err(path, f"invalid JSON: {exc}")
                continue
            if not isinstance(state, dict):
                err(path, "must be a JSON object")
                continue
            for problem in check_state(state, expected_id=path.stem):
                err(path, problem)
            for name in hygiene_problems(text):
                err(path, f"contains a {name}; state must hold no secrets or personal data")
            stats["work"] += 1

    if layout.log.is_dir():
        for path in sorted(layout.log.iterdir()):
            if not re.match(r"^\d{4}-\d{2}\.jsonl$", path.name):
                err(path, "unexpected file; logs are YYYY-MM.jsonl")
                continue
            for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
                if not line.strip():
                    continue
                try:
                    event = json.loads(line)
                except json.JSONDecodeError as exc:
                    err(path, f"line {number}: invalid JSON: {exc}")
                    continue
                for problem in check_event(event, month=path.stem):
                    err(path, f"line {number}: {problem}")
                for name in hygiene_problems(line):
                    err(path, f"line {number}: contains a {name}; the log must hold no secrets or personal data")
                stats["events"] += 1
    return errs, stats


# ---------------------------------------------------------------- commands

def split_list(value: str | None) -> list[str]:
    if not value:
        return []
    return [part.strip() for part in value.split(",") if part.strip()]


def cmd_validate(layout: Layout, args) -> int:
    errs, stats = validate_all(layout)
    if errs:
        for line in errs:
            print(f"error: {line}", file=sys.stderr)
        print(f"memory: FAILED ({len(errs)} problems)", file=sys.stderr)
        return 1
    if stats["stale"]:
        print(f"warning: {stats['stale']} active entries are past review_by (run: stale)")
    print(f"memory: {stats['entries']} entries, {stats['work']} work items, {stats['events']} events, ok")
    return 0


def cmd_index(layout: Layout, args) -> int:
    write_index(layout)
    print(f"wrote {layout.rel(layout.index)}")
    return 0


def template_body(kind: str) -> str:
    parts = []
    for section in SECTIONS[kind]:
        parts.append(f"## {section}\n\nTODO: write this section.\n")
    return "\n".join(parts)


def cmd_new(layout: Layout, args) -> int:
    entries = load_entries(layout)
    numbers = [int(e["meta"]["id"].split("-")[1]) for e in entries if MEM_ID_RE.match(str(e["meta"].get("id", "")))]
    memory_id = f"MEM-{(max(numbers) + 1) if numbers else 1:04d}"
    stamp = fmt_ts(now())
    meta = {
        "id": memory_id, "type": args.type, "title": args.title, "summary": args.summary,
        "topics": split_list(args.topics), "created": stamp, "updated": stamp,
        "author": args.author, "source": args.source, "status": "active",
        "confidence": args.confidence,
    }
    if args.review_by:
        meta["review_by"] = args.review_by
    related = split_list(args.related)
    if related:
        meta["related"] = related
    target = None
    if args.supersedes:
        target = next((e for e in entries if e["meta"]["id"] == args.supersedes), None)
        if target is None:
            raise Fail(f"cannot supersede {args.supersedes}: no such entry")
        meta["supersedes"] = args.supersedes
    errs = check_meta(meta)
    if errs:
        raise Fail("; ".join(errs))
    if args.body == "-":
        body = sys.stdin.read()
    elif args.body:
        body = args.body
    else:
        body = template_body(args.type)
    errs = check_body(meta, body)
    if args.body and errs:
        raise Fail("; ".join(errs))
    problems = hygiene_problems(render_entry(meta, body))
    if problems:
        raise Fail(f"refusing to write: contains a {problems[0]}; memory must hold no secrets or personal data")
    layout.memory.mkdir(parents=True, exist_ok=True)
    path = layout.memory / f"{memory_id}-{slugify(args.title)}.md"
    path.write_text(render_entry(meta, body), encoding="utf-8")
    if target is not None:
        target["meta"]["status"] = "superseded"
        target["meta"]["updated"] = stamp
        target["path"].write_text(render_entry(target["meta"], target["body"]), encoding="utf-8")
    write_index(layout)
    print(layout.rel(path))
    if not args.body:
        print("note: fill in the TODO sections, then run: python3 scripts/memory.py validate")
    return 0


def cmd_update(layout: Layout, args) -> int:
    entries = load_entries(layout)
    entry = next((e for e in entries if e["meta"]["id"] == args.id), None)
    if entry is None:
        raise Fail(f"no entry {args.id}")
    meta = entry["meta"]
    changed = False
    for key, value in (("title", args.title), ("summary", args.summary), ("status", args.status),
                       ("confidence", args.confidence), ("review_by", args.review_by)):
        if value is not None:
            meta[key] = value
            changed = True
    if args.add_topics:
        meta["topics"] = sorted(set(meta["topics"]) | set(split_list(args.add_topics)))
        changed = True
    if args.add_related:
        meta["related"] = sorted(set(meta.get("related", [])) | set(split_list(args.add_related)))
        changed = True
    if not changed:
        raise Fail("nothing to update; pass at least one option")
    meta["updated"] = fmt_ts(now())
    errs = check_meta(meta)
    if errs:
        raise Fail("; ".join(errs))
    if args.status == "superseded":
        print("note: a superseded entry needs another entry with supersedes: " + args.id)
    entry["path"].write_text(render_entry(meta, entry["body"]), encoding="utf-8")
    write_index(layout)
    print(layout.rel(entry["path"]))
    return 0


def matches(entry: dict, args) -> bool:
    meta = entry["meta"]
    if args.status != "any" and meta["status"] != args.status:
        return False
    if args.type and meta["type"] != args.type:
        return False
    if args.topic and args.topic not in meta["topics"]:
        return False
    if args.text:
        needle = args.text.lower()
        haystack = " ".join([meta["title"], meta["summary"], " ".join(meta["topics"]), entry["body"]]).lower()
        if needle not in haystack:
            return False
    return True


def cmd_search(layout: Layout, args) -> int:
    found = [e for e in load_entries(layout) if matches(e, args)]
    for e in sorted(found, key=lambda e: e["meta"]["id"]):
        m = e["meta"]
        print(f"{m['id']}  [{m['type']}/{m['status']}]  {m['title']}: {m['summary']}  ({', '.join(m['topics'])})")
    if not found:
        print("no matching memory")
    return 0


def cmd_stale(layout: Layout, args) -> int:
    today = now().strftime("%Y-%m-%d")
    due = [e for e in load_entries(layout)
           if e["meta"]["status"] == "active" and e["meta"].get("review_by", "9999-12-31") < today]
    for e in sorted(due, key=lambda e: e["meta"]["id"]):
        print(f"{e['meta']['id']}  review_by {e['meta']['review_by']}  {e['meta']['title']}")
    if not due:
        print("no entries are past review_by")
    return 1 if (due and args.fail) else 0


def cmd_log(layout: Layout, args) -> int:
    record = append_event(layout, args.actor, args.event, args.work, args.summary, split_list(args.refs))
    print(json.dumps(record, ensure_ascii=False))
    return 0


def cmd_state(layout: Layout, args) -> int:
    action = args.state_action
    if action == "id":
        print(new_work_id(args.branch))
        return 0
    if action == "list":
        for s in all_states(layout):
            if args.open and s.get("status") in CLOSED_STATUSES:
                continue
            print(f"{s['work_id']} | {s['status']} | tier {s['tier']} | {s['step'] or '-'} | {s['title']}")
        return 0
    if action == "show":
        print(json.dumps(load_state(layout, args.work_id), indent=2, ensure_ascii=False))
        return 0
    if action == "init":
        path = state_path(layout, args.work_id)
        if path.exists():
            raise Fail(f"work state {args.work_id} already exists")
        stamp = fmt_ts(now())
        state = {
            "work_id": args.work_id, "title": args.title, "tier": args.tier, "branch": args.branch,
            "status": args.status, "step": "", "steps_done": [],
            "requirements": split_list(args.requirements), "specs": [], "owner_agent": args.owner_agent,
            "blockers": [], "open_questions": [], "next_actions": [], "pr": None,
            "created": stamp, "updated": stamp,
        }
        save_state(layout, state)
        print(layout.rel(path))
        return 0
    if action == "set":
        state = load_state(layout, args.work_id)
        for pair in args.pairs:
            if "=" not in pair:
                raise Fail(f"expected key=value, got {pair!r}")
            key, value = pair.split("=", 1)
            apply_state_change(state, key, value)
        state["updated"] = fmt_ts(now())
        save_state(layout, state)
        print(layout.rel(state_path(layout, args.work_id)))
        return 0
    raise Fail(f"unknown state action {action}")


ADD_KEYS = {
    "add-step-done": "steps_done", "add-blocker": "blockers", "add-question": "open_questions",
    "add-next": "next_actions", "add-requirement": "requirements", "add-spec": "specs",
}
REMOVE_KEYS = {
    "remove-blocker": "blockers", "remove-question": "open_questions", "remove-next": "next_actions",
}


def apply_state_change(state: dict, key: str, value: str) -> None:
    if key in ("step", "status", "title", "owner-agent", "owner_agent", "branch"):
        state["owner_agent" if key == "owner-agent" else key] = value
    elif key == "tier":
        if not value.isdigit():
            raise Fail("tier must be 0, 1 or 2")
        state["tier"] = int(value)
    elif key == "pr":
        state["pr"] = None if value.lower() in ("", "none", "null") else value
    elif key in ADD_KEYS:
        items = state[ADD_KEYS[key]]
        if value not in items:
            items.append(value)
    elif key in REMOVE_KEYS:
        items = state[REMOVE_KEYS[key]]
        if value not in items:
            raise Fail(f"{key}: {value!r} is not in the list")
        items.remove(value)
    else:
        allowed = ["step", "status", "title", "tier", "branch", "owner-agent", "pr", *ADD_KEYS, *REMOVE_KEYS]
        raise Fail(f"unknown key {key!r}; use one of: {', '.join(allowed)}")


def cmd_summary(layout: Layout, args) -> int:
    """Short, never-failing overview for the session start hook."""
    try:
        entries = load_entries(layout)
        today = now().strftime("%Y-%m-%d")
        active = [e for e in entries if e["meta"]["status"] == "active"]
        due = [e for e in active if e["meta"].get("review_by", "9999-12-31") < today]
        print(f"- Memory: {len(active)} active entries ({layout.rel(layout.index)}); {len(due)} past review_by")
        for s in all_states(layout):
            if s.get("status") not in CLOSED_STATUSES:
                line = f"- Open work: {s['work_id']} [{s['status']}] tier {s['tier']}, step: {s['step'] or '-'}"
                age = (now() - parse_ts(s["updated"])).days if s.get("updated") else None
                if age is not None and age >= STALE_DAYS:
                    line += (f" (stale: not updated for {age} days; the PR is the record. If it merged,"
                             f" close it: state set {s['work_id']} status=done)")
                print(line)
    except Exception:  # the hook must never break a session
        print("- Memory: unreadable, run: python3 scripts/memory.py validate")
    return 0


# ---------------------------------------------------------------- CLI

def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="memory.py", description="Shared agent memory and work state.")
    parser.add_argument("--root", default=os.environ.get("MEMORY_ROOT"), help="repository root (default: this repo)")
    sub = parser.add_subparsers(dest="command", required=True)

    sub.add_parser("validate", help="check memory, work state and the event log")
    sub.add_parser("index", help="regenerate .agent/memory/INDEX.md")
    sub.add_parser("summary", help="short overview for session start")

    new = sub.add_parser("new", help="create a memory entry")
    new.add_argument("--type", required=True, choices=MEMORY_TYPES)
    new.add_argument("--title", required=True)
    new.add_argument("--summary", required=True)
    new.add_argument("--topics", required=True, help="comma separated, lowercase-kebab")
    new.add_argument("--author", default="orchestrator")
    new.add_argument("--source", default="conversation")
    new.add_argument("--confidence", choices=CONFIDENCES, default="medium")
    new.add_argument("--related", help="comma separated IDs: FR-012,ADR-0002,specs/001-x")
    new.add_argument("--review-by", dest="review_by", help="YYYY-MM-DD")
    new.add_argument("--supersedes", help="MEM id this entry replaces (marks it superseded)")
    new.add_argument("--body", help="full Markdown body, or - to read it from stdin")

    upd = sub.add_parser("update", help="change front matter fields of an entry")
    upd.add_argument("id")
    upd.add_argument("--title")
    upd.add_argument("--summary")
    upd.add_argument("--status", choices=MEMORY_STATUSES)
    upd.add_argument("--confidence", choices=CONFIDENCES)
    upd.add_argument("--review-by", dest="review_by")
    upd.add_argument("--add-topics", dest="add_topics")
    upd.add_argument("--add-related", dest="add_related")

    search = sub.add_parser("search", help="find memory entries")
    search.add_argument("--topic")
    search.add_argument("--type", choices=MEMORY_TYPES)
    search.add_argument("--status", choices=(*MEMORY_STATUSES, "any"), default="active")
    search.add_argument("--text")

    stale = sub.add_parser("stale", help="list active entries past review_by")
    stale.add_argument("--fail", action="store_true", help="exit 1 when any are due")

    log = sub.add_parser("log", help="append an event to the log")
    log.add_argument("--actor", required=True)
    log.add_argument("--event", required=True, choices=EVENTS)
    log.add_argument("--work", help="work id")
    log.add_argument("--summary", required=True)
    log.add_argument("--refs", help="comma separated references")

    state = sub.add_parser("state", help="work state files")
    st = state.add_subparsers(dest="state_action", required=True)
    init = st.add_parser("init", help="create a work state")
    init.add_argument("work_id")
    init.add_argument("--title", required=True)
    init.add_argument("--tier", required=True, type=int, choices=TIERS)
    init.add_argument("--branch", required=True)
    init.add_argument("--status", choices=WORK_STATUSES, default="planned")
    init.add_argument("--owner-agent", dest="owner_agent", default="orchestrator")
    init.add_argument("--requirements")
    setp = st.add_parser("set", help="change fields: step=..., status=..., add-blocker=...")
    setp.add_argument("work_id")
    setp.add_argument("pairs", nargs="+", metavar="key=value")
    show = st.add_parser("show", help="print a work state")
    show.add_argument("work_id")
    lst = st.add_parser("list", help="list work states")
    lst.add_argument("--open", action="store_true", help="hide done and abandoned")
    ident = st.add_parser("id", help="print the work id for a branch name")
    ident.add_argument("branch")
    return parser


COMMANDS = {
    "validate": cmd_validate, "index": cmd_index, "new": cmd_new, "update": cmd_update,
    "search": cmd_search, "stale": cmd_stale, "log": cmd_log, "state": cmd_state, "summary": cmd_summary,
}


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    root = Path(args.root).resolve() if args.root else Path(__file__).resolve().parent.parent
    try:
        return COMMANDS[args.command](Layout(root), args)
    except Fail as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
