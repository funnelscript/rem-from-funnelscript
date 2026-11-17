#!/usr/bin/env python3
"""Initialize and check a REM ledger using the Python standard library."""

import argparse
import json
import re
import sys
from graphlib import CycleError, TopologicalSorter
from pathlib import Path, PurePosixPath


TEMPLATES = Path(__file__).resolve().parent.parent / "templates"
LEDGER_FILES = ("STATUS.md", "EVIDENCE.md", "DECISIONS.md", "PARITY.md", "TASKS.json")
TASK_STATUSES = {"pending", "ready", "in_progress", "blocked", "done"}
EVIDENCE_ID = re.compile(r"(?:EV|E)-[0-9]+")
EVIDENCE_ROW = re.compile(r"^\s*\|\s*((?:EV|E)-[0-9]+)\s*\|", re.MULTILINE)


def initialize(ledger):
    ledger.mkdir(parents=True, exist_ok=True)
    for name in LEDGER_FILES:
        try:
            with (ledger / name).open("xb") as output:
                output.write((TEMPLATES / name).read_bytes())
        except FileExistsError:
            continue
        print(f"Created {ledger / name}")


def is_text(value):
    return isinstance(value, str) and bool(value.strip())


def is_text_list(value):
    return isinstance(value, list) and all(is_text(item) for item in value)


def path_parts(path):
    if path in {".", ".."} or path.startswith("-") or "\\" in path or ":" in path:
        return ()
    parts = PurePosixPath(path).parts
    if not parts or path.startswith("/") or ".." in parts:
        return ()
    return parts


def check_tasks(tasks, evidence_ids, errors):
    graph = {}
    active_paths = []
    statuses = {}
    for index, task in enumerate(tasks, 1):
        if not isinstance(task, dict) or not is_text(task.get("id")):
            errors.append(f"Task {index} needs a non-empty id")
            continue
        task_id = task["id"]
        if task_id in graph:
            errors.append(f"Duplicate task id: {task_id}")
            continue

        depends_on = task.get("depends_on")
        references = task.get("evidence_ids")
        paths = task.get("write_paths")
        checks = task.get("checks")
        status = task.get("status")

        if not is_text(task.get("owner")) or not is_text(task.get("outcome")):
            errors.append(f"{task_id}: owner and outcome are required")
        if not is_text(status) or status not in TASK_STATUSES:
            errors.append(f"{task_id}: invalid status")
        if not is_text_list(depends_on):
            errors.append(f"{task_id}: depends_on must be a list of task IDs")
            depends_on = []
        if not is_text_list(references):
            errors.append(f"{task_id}: evidence_ids must be a list of evidence IDs")
        else:
            for ref in references:
                if not EVIDENCE_ID.fullmatch(ref) or ref not in evidence_ids:
                    errors.append(f"{task_id}: unknown evidence ID {ref}")
        if not is_text_list(checks) or not checks:
            errors.append(f"{task_id}: at least one verification command is required")
        if not is_text_list(paths) or any(not path_parts(path) for path in paths):
            errors.append(f"{task_id}: write_paths must contain project-relative paths")
            paths = []

        graph[task_id] = tuple(depends_on)
        statuses[task_id] = status
        if status == "done" and not is_text(task.get("verification")):
            errors.append(f"{task_id}: completed tasks require a verification reference")
        if status in ("ready", "in_progress"):
            for path in paths:
                active_paths.append((task_id, path_parts(path)))

    for task_id, dependencies in graph.items():
        missing = set(dependencies) - graph.keys()
        if missing:
            errors.append(f"{task_id}: unknown dependencies {', '.join(sorted(missing))}")
        if statuses[task_id] in ("ready", "in_progress", "done") and any(
            statuses.get(dependency) != "done" for dependency in dependencies
        ):
            errors.append(f"{task_id}: {statuses[task_id]} task has unfinished dependencies")
    try:
        tuple(TopologicalSorter(graph).static_order())
    except CycleError as exc:
        errors.append(f"Dependency cycle: {' -> '.join(exc.args[1])}")

    for index, (task_id, path) in enumerate(active_paths):
        for other_id, other_path in active_paths[index + 1:]:
            if task_id == other_id:
                continue
            depth = min(len(path), len(other_path))
            if path[:depth] == other_path[:depth]:
                errors.append(
                    f"Overlapping write paths: {task_id} ({'/'.join(path)}) and "
                    f"{other_id} ({'/'.join(other_path)})"
                )


def check(ledger):
    missing = [name for name in LEDGER_FILES if not (ledger / name).is_file()]
    if missing:
        return [f"Missing ledger file: {name}" for name in missing], 0
    tasks = json.loads((ledger / "TASKS.json").read_text(encoding="utf-8"))
    if (
        not isinstance(tasks, dict)
        or type(tasks.get("version")) is not int
        or tasks["version"] != 1
        or not isinstance(tasks.get("tasks"), list)
    ):
        return ["TASKS.json must have version 1 and a tasks array"], 0

    evidence = EVIDENCE_ROW.findall((ledger / "EVIDENCE.md").read_text(encoding="utf-8"))
    errors = []
    if len(set(evidence)) != len(evidence):
        errors.append("Duplicate evidence ID in EVIDENCE.md")
    check_tasks(tasks["tasks"], set(evidence), errors)
    return errors, len(tasks["tasks"])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("init", "check"))
    parser.add_argument("--root", type=Path, default=Path.cwd(), help="project root")
    parser.add_argument("--kb", type=Path, default=None, help="optional path to a SaaS knowledge base to validate")
    args = parser.parse_args()

    if args.kb:
        import subprocess
        bin_path = args.root.resolve() / "rem-from-funnelscript" / "rem"
        if not bin_path.is_file():
            bin_path = Path("rem")
        try:
            cmd = [str(bin_path), "kb", args.action, str(args.kb)]
            res = subprocess.run(cmd, capture_output=True, text=True)
            if res.returncode == 0:
                print(res.stdout, end="")
                return 0
            else:
                print(res.stderr, file=sys.stderr, end="")
                return res.returncode
        except FileNotFoundError:
            pass

    ledger = args.root.resolve() / ".re-ledger"
    try:
        if args.action == "init":
            if not ledger.parent.is_dir():
                raise ValueError(f"Project root is not a directory: {ledger.parent}")
            if ledger.is_symlink():
                raise ValueError(f"Expected a local ledger directory: {ledger}")
            initialize(ledger)
            return 0
        errors, task_count = check(ledger)
    except (OSError, ValueError) as exc:
        print(exc, file=sys.stderr)
        return 1

    if errors:
        print("\n".join(errors), file=sys.stderr)
        return 1
    print(f"Ledger structure OK ({task_count} tasks)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
