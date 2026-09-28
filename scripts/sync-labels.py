#!/usr/bin/env python3
"""Apply .github/labels.yml to the repository through `gh`.

    python3 scripts/sync-labels.py            # create or update every label
    python3 scripts/sync-labels.py --check    # exit 1 if any differs, write nothing

labels.yml is a declaration; GitHub holds the live state and nothing else
syncs it. This is the one way the two are brought together. Non-destructive:
a label on the repository that the file does not name is left alone, since a
repo may carry labels this file does not govern.

No PyYAML: the file is a flat list of {name, color, description} entries and
comments, and a parser for exactly that shape is shorter than a dependency.
"""

from __future__ import annotations

import json
import pathlib
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
LABELS = ROOT / ".github" / "labels.yml"


def unquote(value: str) -> str:
    value = value.strip()
    if len(value) >= 2 and value[0] == value[-1] and value[0] in "'\"":
        return value[1:-1]
    return value


def parse(text: str) -> list[dict[str, str]]:
    labels: list[dict[str, str]] = []
    current: dict[str, str] | None = None
    for raw in text.splitlines():
        line = raw.split("#", 1)[0] if raw.lstrip().startswith("#") else raw
        if not line.strip():
            continue
        if line.startswith("- "):
            current = {}
            labels.append(current)
            line = "  " + line[2:]
        if current is None or ":" not in line:
            raise SystemExit(f"labels.yml: cannot read line: {raw!r}")
        key, value = line.strip().split(":", 1)
        current[key.strip()] = unquote(value)
    for label in labels:
        for key in ("name", "color", "description"):
            if key not in label:
                raise SystemExit(f"labels.yml: {label.get('name', '?')} has no {key}")
    return labels


def gh(*args: str) -> str:
    result = subprocess.run(["gh", *args], capture_output=True, text=True, check=False)
    if result.returncode != 0:
        raise SystemExit(f"gh {' '.join(args)}: {result.stderr.strip()}")
    return result.stdout


def main() -> int:
    check = "--check" in sys.argv[1:]
    wanted = parse(LABELS.read_text(encoding="utf-8"))
    live = {
        item["name"]: item
        for item in json.loads(
            gh("label", "list", "--limit", "200", "--json", "name,color,description")
        )
    }
    drift = 0
    for label in wanted:
        have = live.get(label["name"])
        same = (
            have is not None
            and have["color"].lower() == label["color"].lower()
            and have["description"] == label["description"]
        )
        if same:
            continue
        drift += 1
        verb = "update" if have else "create"
        print(f"{verb}: {label['name']}")
        if not check:
            gh(
                "label",
                "create",
                label["name"],
                "--force",
                "--color",
                label["color"],
                "--description",
                label["description"],
            )
    if drift == 0:
        print(f"{len(wanted)} labels in step")
    return 1 if (check and drift) else 0


if __name__ == "__main__":
    sys.exit(main())
