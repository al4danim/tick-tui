#!/usr/bin/env python3
"""One-shot ID migration for tick-tui v0.2 → v0.3.

Reassigns every task's ID from the old sequential integer format to a fresh
8-char random hex. Necessary because two devices (Mac CLI + iOS Obsidian
plugin) both compute "max + 1" and clash when their views of tasks.md aren't
in sync. Random IDs make collision practically impossible.

Also moves the data directory from ~/hoard/tick/ → ~/hoard/.tick/ so it stops
showing up in Obsidian's file tree (where stray edits keep poking holes in
the format).

Usage:
    python3 scripts/renum.py
    python3 scripts/renum.py --src ~/hoard/tick --dst ~/hoard/.tick
"""

from __future__ import annotations

import argparse
import re
import secrets
import shutil
import sys
from pathlib import Path

ID_RE = re.compile(r"\s\[(\d+)\]\s*$")


def new_id() -> str:
    return secrets.token_hex(4)  # 8 hex chars


def renum_lines(lines: list[str], used: set[str]) -> list[str]:
    out = []
    for line in lines:
        m = ID_RE.search(line)
        if not m:
            out.append(line)
            continue
        # Replace the trailing [<digits>] with a fresh hex ID, ensuring the
        # new ID isn't already taken by another row.
        nid = new_id()
        while nid in used:
            nid = new_id()
        used.add(nid)
        out.append(line[: m.start()] + f" [{nid}]")
    return out


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--src", default=str(Path.home() / "hoard" / "tick"))
    ap.add_argument("--dst", default=str(Path.home() / "hoard" / ".tick"))
    args = ap.parse_args()

    src = Path(args.src).expanduser()
    dst = Path(args.dst).expanduser()

    if not src.exists():
        # Maybe already renamed; just renum in place at dst.
        if dst.exists():
            print(f"src missing, renumbering in place at {dst}", file=sys.stderr)
        else:
            print(f"neither {src} nor {dst} exists; nothing to migrate", file=sys.stderr)
            return 1
    else:
        if dst.exists() and dst != src:
            print(f"dst {dst} already exists; aborting to avoid clobber", file=sys.stderr)
            return 1
        if dst != src:
            print(f"moving {src} → {dst}")
            shutil.move(str(src), str(dst))

    used: set[str] = set()
    for name in ("tasks.md", "archive.md"):
        path = dst / name
        if not path.exists():
            continue
        original = path.read_text().splitlines(keepends=True)
        # Strip line endings from each line, renum, restore.
        bare = [ln.rstrip("\n") for ln in original]
        renumbered = renum_lines(bare, used)
        # Preserve trailing-newline status of the original file.
        end = "\n" if original and original[-1].endswith("\n") else ""
        path.write_text("\n".join(renumbered) + end)
        n_changed = sum(1 for a, b in zip(bare, renumbered) if a != b)
        print(f"{path}: {n_changed}/{len(bare)} rows renumbered")

    print(f"\ndone. update ~/.config/tick/config to point at {dst}/tasks.md")
    return 0


if __name__ == "__main__":
    sys.exit(main())
