#!/usr/bin/env python3
"""work.py — tracking tooling for work/ (see work/README.md).

Stdlib only: a tiny YAML front-matter parser is embedded, no pip packages.
The tracking files are the source of truth; this script only checks and
indexes them.

Usage:
  python3 scripts/work.py validate|board|view|next
  python3 scripts/work.py start --id VYQ-10
  python3 scripts/work.py done  --id VYQ-10
  python3 scripts/work.py new --type task --epic VYQ-2 --area go --title "..."
"""

import argparse
import glob
import json
import os
import re
import sys
import webbrowser
from datetime import date

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

STATUSES = ["open", "in-progress", "blocked", "done"]
PRIORITIES = ["critical", "major", "normal", "minor"]
AREAS = ["setup", "docs", "go", "contract", "js", "ops"]
PHASE_ORDER = ["F0", "F1", "F2", "F3", "F4", "F5", "F6"]

TASK_REQUIRED = ["id", "type", "title", "status", "priority", "area",
                 "epic", "depends_on", "blocked_reason", "updated"]
EPIC_REQUIRED = ["id", "type", "title", "phase", "depends_on", "updated"]

WIP_LIMIT = 1  # one task in progress at a time


# ---------------------------------------------------------------- YAML mini

def parse_scalar(text):
    text = text.strip()
    if text == "" or text == "null" or text == "~":
        return None
    if text == "[]":
        return []
    if text.startswith("[") and text.endswith("]"):
        inner = text[1:-1].strip()
        if not inner:
            return []
        return [item.strip().strip('"').strip("'") for item in inner.split(",")]
    if len(text) >= 2 and text[0] == text[-1] and text[0] in "\"'":
        return text[1:-1]
    return text


def parse_front(raw):
    """Parse YAML front matter. Returns (dict, body). Raises ValueError."""
    if not raw.startswith("---\n"):
        raise ValueError("missing YAML front matter")
    close = raw.find("\n---\n", 3)
    if close < 0:
        raise ValueError("unterminated YAML front matter")
    data = {}
    for line in raw[4:close].splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        key, sep, value = line.partition(":")
        if not sep:
            raise ValueError(f"invalid front matter line: {line!r}")
        data[key.strip()] = parse_scalar(value)
    return data, raw[close + 5:]


def dump_scalar(value):
    if value is None:
        return "null"
    if isinstance(value, list):
        if not value:
            return "[]"
        return "[" + ", ".join(str(v) for v in value) + "]"
    if isinstance(value, str) and (value != value.strip() or ":" in value
                                   or value.startswith("[") or value == ""
                                   or value in STATUSES + PRIORITIES + AREAS
                                   or value.lower() in ("null", "true", "false")):
        escaped = value.replace('"', '\\"')
        return f'"{escaped}"'
    return str(value)


def dump_front(data, order):
    lines = ["---"]
    for key in order:
        lines.append(f"{key}: {dump_scalar(data.get(key))}")
    extra = [k for k in data if k not in order]
    for key in extra:
        lines.append(f"{key}: {dump_scalar(data[key])}")
    return "\n".join(lines) + "\n---\n"


# ---------------------------------------------------------------- loading

class Item:
    def __init__(self, path, kind, front, body):
        self.path = path
        self.kind = kind
        self.front = front
        self.body = body
        self.id = str(front.get("id", ""))
        m = re.search(r"\d+", self.id)
        self.num = int(m.group(0)) if m else 0

    @property
    def rel(self):
        return os.path.relpath(self.path, ROOT)

    @property
    def rel_work(self):
        return os.path.relpath(self.path, os.path.join(ROOT, "work"))

    @property
    def status(self):
        return str(self.front.get("status", ""))

    def get(self, key, default=None):
        return self.front.get(key, default)


def load_items():
    items = []
    work = os.path.join(ROOT, "work")
    for pattern, kind in [(os.path.join(work, "epics", "*.md"), "epic"),
                          (os.path.join(work, "tasks", "*", "*.md"), "task")]:
        for path in sorted(glob.glob(pattern)):
            with open(path, encoding="utf-8") as handle:
                raw = handle.read()
            front, body = parse_front(raw)
            items.append(Item(path, kind, front, body))
    return items


def section(body, name):
    lines = body.splitlines()
    start = next((i for i, l in enumerate(lines)
                  if l.startswith("## ") and name.lower() in l.lower()), None)
    if start is None:
        return ""
    out = []
    for line in lines[start + 1:]:
        if line.startswith("## "):
            break
        out.append(line)
    return "\n".join(out)


def verification_boxes(body):
    return re.findall(r"^\s*- \[( |x|X)\]",
                      section(body, "verification"), re.MULTILINE)


def blocks_of(items, item):
    """Reverse edges derived in memory: who depends on this item."""
    return [other.id for other in items
            if item.id in [str(d) for d in (other.get("depends_on") or [])]]


def slugify(title, words=6):
    title = re.sub(r"^\[[^\]]*\]\s*", "", title)
    slug = re.sub(r"[^a-z0-9]+", "-", title.lower()).strip("-")
    return "-".join(slug.split("-")[:words])


# ---------------------------------------------------------------- validator

class Validator:
    def __init__(self, items):
        self.items = items
        self.by_id = {i.id: i for i in items}
        self.errors = []
        self.warnings = []

    def err(self, item, msg):
        self.errors.append(f"{item.rel}: {msg}")

    def run(self):
        self._duplicates()
        for item in self.items:
            self._item(item)
        self._cycles()
        self._next_id()
        self._board()
        return self

    def _duplicates(self):
        seen = {}
        for item in self.items:
            seen.setdefault(item.id, []).append(item.rel)
        for iid, paths in seen.items():
            if len(paths) > 1:
                self.errors.append(f"duplicate id {iid}: {', '.join(paths)}")

    def _item(self, item):
        required = TASK_REQUIRED if item.kind == "task" else EPIC_REQUIRED
        missing = [k for k in required if k not in item.front]
        if missing:
            self.err(item, f"missing front matter keys: {', '.join(missing)}")
        unknown = [k for k in item.front if k not in required]
        if unknown:
            self.err(item, f"unknown front matter keys: {', '.join(unknown)}")

        if not re.fullmatch(r"VYQ-\d+", item.id):
            self.err(item, f"id {item.id!r} must match VYQ-<n>")
            return
        if not os.path.basename(item.path).startswith(item.id + "-"):
            self.err(item, f"filename must start with {item.id}-")
        if str(item.front.get("type")) != item.kind:
            self.err(item, f"type must be {item.kind}")
        if not str(item.front.get("title", "")).strip():
            self.err(item, "title is empty")
        if not re.fullmatch(r"\d{4}-\d{2}-\d{2}", str(item.front.get("updated", ""))):
            self.err(item, f"updated must be YYYY-MM-DD, got {item.front.get('updated')!r}")

        deps = item.get("depends_on")
        if not isinstance(deps, list):
            self.err(item, "depends_on must be a list")
            deps = []
        for dep in deps:
            if str(dep) == item.id:
                self.err(item, "depends_on references itself")
            elif str(dep) not in self.by_id:
                self.err(item, f"depends_on references unknown {dep}")

        if item.kind == "task":
            self._task(item, deps)
        else:
            self._epic(item)

    def _task(self, item, deps):
        if item.status not in STATUSES:
            statuses_str = "|".join(STATUSES)
            self.err(item, f"status must be one of {statuses_str}")
        if str(item.front.get("priority")) not in PRIORITIES:
            priorities_str = "|".join(PRIORITIES)
            self.err(item, f"priority must be one of {priorities_str}")
        if str(item.front.get("area")) not in AREAS:
            areas_str = "|".join(AREAS)
            self.err(item, f"area must be one of {areas_str}")

        epic = self.by_id.get(str(item.front.get("epic")))
        if not epic or epic.kind != "epic":
            self.err(item, f"epic {item.front.get('epic')} does not exist")
        elif os.path.basename(os.path.dirname(item.path)) != epic.id:
            self.err(item, f"must live in work/tasks/{epic.id}/")

        if item.status == "blocked" and not str(item.front.get("blocked_reason") or "").strip():
            self.err(item, "blocked requires blocked_reason")

        if item.status in ("in-progress", "done"):
            for dep in deps:
                other = self.by_id.get(str(dep))
                if other and other.kind == "task" and other.status != "done":
                    self.err(item, f"depends on {dep} which is {other.status}")

        if item.status == "done":
            boxes = verification_boxes(item.body)
            if not boxes:
                self.warnings.append(f"{item.rel}: done but Verification has no checklist")
            elif any(b == " " for b in boxes):
                open_count = boxes.count(" ")
                self.err(item, f"status done requires every Verification box ticked ({open_count} open)")

    def _epic(self, item):
        phase_order_str = "|".join(PHASE_ORDER)
        if str(item.front.get("phase")) not in PHASE_ORDER:
            self.err(item, f"phase must be one of {phase_order_str}")
        children = [i for i in self.items
                    if i.kind == "task" and str(i.get("epic")) == item.id]
        if not children:
            self.warnings.append(f"{item.rel}: epic has no tasks")

    def _cycles(self):
        state, stack = {}, []

        def visit(iid):
            if state.get(iid) == 2:
                return
            if state.get(iid) == 1:
                cycle_str = " -> ".join(stack[stack.index(iid):] + [iid])
                self.errors.append(f"dependency cycle: {cycle_str}")
                return
            state[iid] = 1
            stack.append(iid)
            node = self.by_id.get(iid)
            for dep in (node.get("depends_on") or []) if node else []:
                visit(str(dep))
            stack.pop()
            state[iid] = 2

        for iid in self.by_id:
            visit(iid)

    def _next_id(self):
        path = os.path.join(ROOT, "work", "NEXT-ID")
        if not os.path.exists(path):
            self.errors.append("work/NEXT-ID is missing")
            return
        with open(path, encoding="utf-8") as handle:
            nxt = int(handle.read().strip())
        highest = max((i.num for i in self.items), default=0)
        if nxt <= highest:
            self.errors.append(f"work/NEXT-ID is {nxt}, but VYQ-{highest} already exists")

    def _board(self):
        path = os.path.join(ROOT, "work", "BOARD.md")
        if not os.path.exists(path):
            self.errors.append("work/BOARD.md is missing — run make work-board")
            return
        with open(path, encoding="utf-8") as handle:
            current = handle.read()
        if current != render_board(self.items):
            self.errors.append("work/BOARD.md is stale — run make work-board")


# ---------------------------------------------------------------- board.md

def tasks_of(items, epic):
    return sorted([t for t in items
                   if t.kind == "task" and str(t.get("epic")) == epic.id],
                  key=lambda t: t.num)


def derived_epic_status(tasks):
    statuses = [t.status for t in tasks]
    if not statuses:
        return "open"
    if all(s == "done" for s in statuses):
        return "done"
    if any(s != "open" for s in statuses):
        return "in-progress"
    return "open"


def esc(text):
    return str(text).replace("|", "\\|")


def render_board(items):
    epics = sorted([i for i in items if i.kind == "epic"], key=lambda e: e.num)
    tasks = [i for i in items if i.kind == "task"]

    out = ["<!-- Generated by `make work-board`. Do not edit by hand. -->",
           "", "# Board", "",
           "Execution index for `work/`. Rules in `work/README.md`.", "",
           "## Summary", "", "| State | Tasks |", "|---|---|"]
    for s in STATUSES:
        n = sum(1 for t in tasks if t.status == s)
        if n:
            out.append(f"| `{s}` | {n} |")
    total_tasks = len(tasks)
    out.append(f"| **total** | **{total_tasks}** |")

    out += ["", "## By area", "", "| Area | Tasks | Done |", "|---|---|---|"]
    for area in AREAS:
        owned = [t for t in tasks if str(t.get("area")) == area]
        if owned:
            owned_len = len(owned)
            done_len = sum(1 for t in owned if t.status == "done")
            out.append(f"| `{area}` | {owned_len} | {done_len} |")

    for epic in epics:
        tasks_e = tasks_of(items, epic)
        epic_title = esc(epic.get("title"))
        derived_status = derived_epic_status(tasks_e)
        out += ["", f"## {epic.id} — {epic_title}", "",
                f"- File: [{epic.rel_work}]({epic.rel_work})",
                f"- Derived status: `{derived_status}`", "",
                "| Task | State | Area | Priority | Title |",
                "|---|---|---|---|---|"]
        for t in tasks_e:
            title_esc = esc(t.get("title"))
            out.append(f"| [{t.id}]({t.rel_work}) | `{t.status}` | `{t.get('area')}` | {t.get('priority')} | {title_esc} |")

    blocked = [t for t in tasks if t.status == "blocked"]
    if blocked:
        out += ["", "## Blocked", ""]
        for t in blocked:
            title_esc = esc(t.get("title"))
            out.append(f"- **{t.id}** {title_esc} — {t.get('blocked_reason')}")

    return "\n".join(out) + "\n"


def cmd_board(items):
    path = os.path.join(ROOT, "work", "BOARD.md")
    with open(path, "w", encoding="utf-8") as handle:
        handle.write(render_board(items))
    print("work: wrote work/BOARD.md")
    return 0


# ---------------------------------------------------------------- html view

def build_view_data(items):
    epics = sorted([i for i in items if i.kind == "epic"], key=lambda e: e.num)
    tasks = sorted([i for i in items if i.kind == "task"], key=lambda t: t.num)
    by_id = {i.id: i for i in items}

    data = {"generated": str(date.today()), "limits": {"tasks": WIP_LIMIT},
            "epics": [], "tasks": []}

    for e in epics:
        own = tasks_of(items, e)
        data["epics"].append({
            "id": e.id, "num": e.num, "phase": str(e.get("phase")),
            "title": str(e.get("title")), "state": derived_epic_status(own),
            "dependsOn": [str(d) for d in (e.get("depends_on") or [])],
            "file": e.rel_work, "total": len(own),
            "counts": {s: sum(1 for t in own if t.status == s) for s in STATUSES},
        })

    for t in tasks:
        deps = [str(d) for d in (t.get("depends_on") or [])]
        blockers = []
        if t.status == "open":
            for dep in deps:
                other = by_id.get(dep)
                if other and other.status != "done":
                    blockers.append(f"Waiting on {dep} ({other.status})")
        data["tasks"].append({
            "id": t.id, "num": t.num,
            "title": re.sub(r"^\[[^\]]*\]\s*", "", str(t.get("title"))),
            "status": t.status, "priority": str(t.get("priority")),
            "area": str(t.get("area")), "epic": str(t.get("epic")),
            "dependsOn": deps, "blocks": blocks_of(items, t),
            "blockedReason": t.get("blocked_reason"),
            "ready": t.status == "open" and not blockers,
            "blockers": blockers, "file": t.rel_work,
            "verification": [b != " " for b in verification_boxes(t.body)],
        })
    return data


def cmd_view(items, do_open):
    cmd_board(items)
    result = Validator(items).run()
    data = build_view_data(items)
    data["validation"] = {"errors": len(result.errors),
                          "warnings": len(result.warnings)}

    template_path = os.path.join(ROOT, "scripts", "work_view.html")
    with open(template_path, encoding="utf-8") as handle:
        template = handle.read()
    payload = json.dumps(data).replace("</", "<\\/")
    html = template.replace("__DATA__", payload)

    out = os.path.join(ROOT, "work", "board.html")
    with open(out, "w", encoding="utf-8") as handle:
        handle.write(html)
    tasks_len = len(data["tasks"])
    epics_len = len(data["epics"])
    print(f"work: wrote work/board.html ({tasks_len} tasks, {epics_len} epics)")
    if do_open:
        webbrowser.open("file://" + out)
    return 0


# ---------------------------------------------------------------- commands

def cmd_validate(items):
    result = Validator(items).run()
    for w in result.warnings:
        print(f"warning: {w}")
    if result.errors:
        for e in result.errors:
            print(f"error: {e}")
        err_len = len(result.errors)
        print(f"work: {err_len} errors")
        return 1
    items_len = len(items)
    warn_len = len(result.warnings)
    print(f"work: OK ({items_len} items, {warn_len} warnings)")
    return 0


def cmd_next(items):
    by_id = {i.id: i for i in items}
    in_flight = [t for t in items
                 if t.kind == "task" and t.status == "in-progress"]
    if in_flight:
        print("In flight — finish before starting anything new:")
        for t in in_flight:
            print(f"  {t.id} {t.get('title')}")
        first_id = in_flight[0].id
        print(f"\nClose with: make work-done ID={first_id}")
        return 0

    epics = {e.id: e for e in items if e.kind == "epic"}
    ready = []
    for t in items:
        if t.kind != "task" or t.status != "open":
            continue
        deps = [str(d) for d in (t.get("depends_on") or [])]
        if all(by_id.get(d) and by_id[d].status == "done" for d in deps):
            ready.append(t)

    if not ready:
        open_tasks = [t for t in items if t.kind == "task" and t.status == "open"]
        blocked = [t for t in items if t.kind == "task" and t.status == "blocked"]
        print("work: no ready tasks.")
        if open_tasks:
            print("  open tasks are waiting on dependencies:")
            for t in sorted(open_tasks, key=lambda t: t.num):
                pending = [str(d) for d in (t.get("depends_on") or [])
                           if by_id.get(str(d)) and by_id[str(d)].status != "done"]
                pending_str = ", ".join(pending)
                print(f"    {t.id} waits on {pending_str}")
        if blocked:
            blocked_str = ", ".join(t.id for t in blocked)
            print(f"  blocked: {blocked_str}")
        return 0

    def rank(t):
        epic = epics.get(str(t.get("epic")))
        phase = str(epic.get("phase")) if epic else "F9"
        return (PHASE_ORDER.index(phase) if phase in PHASE_ORDER else 99,
                PRIORITIES.index(str(t.get("priority"))),
                -len(blocks_of(items, t)), t.num)

    ranked = sorted(ready, key=rank)
    top = ranked[0]
    print(f"-- Recommended: {top.id} --")
    print(f"  Title:   {top.get('title')}")
    print(f"  Area:    {top.get('area')}  Priority: {top.get('priority')}")
    print(f"  Epic:    {top.get('epic')}")
    print(f"  File:    {top.rel}")
    print(f"\nStart with: make work-start ID={top.id}")
    if len(ranked) > 1:
        alt_str = ", ".join(t.id for t in ranked[1:4])
        print(f"\nAlternatives: {alt_str}")
    return 0


def _update_front(item, changes):
    with open(item.path, encoding="utf-8") as handle:
        raw = handle.read()
    front, body = parse_front(raw)
    front.update(changes)
    order = TASK_REQUIRED if item.kind == "task" else EPIC_REQUIRED
    with open(item.path, "w", encoding="utf-8") as handle:
        handle.write(dump_front(front, order) + body)


def _check_wip(items):
    in_flight = [t for t in items
                 if t.kind == "task" and t.status == "in-progress"]
    if len(in_flight) >= WIP_LIMIT:
        in_flight_len = len(in_flight)
        tasks_str = ", ".join(t.id for t in in_flight)
        print(f"work: {in_flight_len} task(s) already in progress: {tasks_str}. Close or reopen "
              "them first.")
        sys.exit(1)


def cmd_start(items, task_id):
    task = next((i for i in items if i.kind == "task" and i.id == task_id), None)
    if not task:
        print(f"work: unknown task {task_id}")
        return 1
    if task.status != "open":
        print(f"work: {task_id} is {task.status}, expected open")
        return 1

    by_id = {i.id: i for i in items}
    pending = [str(d) for d in (task.get("depends_on") or [])
               if by_id.get(str(d)) and by_id[str(d)].status != "done"]
    if pending:
        pending_str = ", ".join(pending)
        print(f"work: {task_id} depends on unfinished work: {pending_str}")
        return 1

    _check_wip(items)
    _update_front(task, {"status": "in-progress", "updated": str(date.today())})
    cmd_board(load_items())
    print(f"work: {task_id} is now in-progress")
    boxes = section(task.body, "verification").splitlines()
    boxes = [l for l in boxes if re.match(r"^\s*- \[", l)]
    if boxes:
        print("\nEvidence required to close:")
        for line in boxes:
            print(f"  {line}")
    return 0


def cmd_done(items, task_id):
    task = next((i for i in items if i.kind == "task" and i.id == task_id), None)
    if not task:
        print(f"work: unknown task {task_id}")
        return 1
    if task.status != "in-progress":
        print(f"work: {task_id} is {task.status}, expected in-progress")
        return 1

    boxes = verification_boxes(task.body)
    if not boxes:
        print(f"work: {task_id} has no Verification checklist")
        return 1
    open_boxes = boxes.count(" ")
    if open_boxes:
        print(f"work: {task_id} has {open_boxes} unticked Verification box(es). Produce the "
              "evidence first.")
        return 1

    _update_front(task, {"status": "done", "updated": str(date.today())})
    cmd_board(load_items())
    print(f"work: {task_id} is done")
    return 0


def cmd_new(args):
    if args.type not in ("task", "epic"):
        print("work: --type must be task or epic")
        return 1
    if not args.title:
        print("work: --title is required")
        return 1

    next_path = os.path.join(ROOT, "work", "NEXT-ID")
    with open(next_path, encoding="utf-8") as handle:
        num = int(handle.read().strip())
    item_id = f"VYQ-{num}"

    if args.type == "task":
        if not args.epic:
            print("work: --epic is required for a task")
            return 1
        directory = os.path.join(ROOT, "work", "tasks", args.epic)
        if not os.path.isdir(directory):
            print(f"work: unknown epic {args.epic}")
            return 1
    else:
        directory = os.path.join(ROOT, "work", "epics")

    slugified_title = slugify(args.title)
    path = os.path.join(directory, f"{item_id}-{slugified_title}.md")
    if os.path.exists(path):
        print(f"work: {path} already exists")
        return 1

    today = str(date.today())
    if args.type == "task":
        area = args.area if args.area in AREAS else "setup"
        front = {"id": item_id, "type": "task", "title": args.title,
                 "status": "open", "priority": "normal", "area": area,
                 "epic": args.epic, "depends_on": [], "blocked_reason": None,
                 "updated": today}
        body = f"\n# {item_id} — {args.title}\n\n## Scope\n\nTODO\n\n## Verification\n\n" \
               "- [ ] TODO evidence\n\n## Evidence & Notes\n\n"
    else:
        front = {"id": item_id, "type": "epic", "title": args.title,
                 "phase": args.phase or "F0", "depends_on": [], "updated": today}
        body = f"\n# {item_id} — {args.title}\n\n## Goal\n\nTODO\n\n## Done criteria\n\nTODO\n"

    content = (
        dump_front(front, TASK_REQUIRED if args.type == "task" else EPIC_REQUIRED)
        + body
    )
    with open(path, "w", encoding="utf-8") as handle:
        handle.write(content)
    with open(next_path, "w", encoding="utf-8") as handle:
        handle.write(f"{num + 1}\n")
    rel_path = os.path.relpath(path, ROOT)
    next_num = num + 1
    print(f"work: created {rel_path} (NEXT-ID -> {next_num})")
    return 0


# ---------------------------------------------------------------- main

def main():
    parser = argparse.ArgumentParser(prog="work.py")
    parser.add_argument("command",
                        choices=["validate", "board", "view", "next",
                                 "start", "done", "new"])
    parser.add_argument("--id")
    parser.add_argument("--type", default="task")
    parser.add_argument("--epic")
    parser.add_argument("--area")
    parser.add_argument("--title")
    parser.add_argument("--phase")
    parser.add_argument("--open", action="store_true")
    args = parser.parse_args()

    items = load_items()

    if args.command == "validate":
        return cmd_validate(items)
    if args.command == "board":
        return cmd_board(items)
    if args.command == "view":
        return cmd_view(items, args.open)
    if args.command == "next":
        return cmd_next(items)
    if args.command == "start":
        if not args.id:
            print("work: --id is required")
            return 1
        return cmd_start(items, args.id)
    if args.command == "done":
        if not args.id:
            print("work: --id is required")
            return 1
        return cmd_done(items, args.id)
    if args.command == "new":
        return cmd_new(args)
    return 2


if __name__ == "__main__":
    sys.exit(main())
