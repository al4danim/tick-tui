#!/usr/bin/env python3
"""One-shot SQLite -> markdown migration for tick-tui.

Reads ~/Library/Application Support/Tick/data.db (the old feature-check store)
and writes:
  <out>/tasks.md       — undone + done within last 7 days
  <out>/archive.md     — done older than 7 days

Usage:
  python3 scripts/migrate.py [--out ~/hoard/tick] [--db <path>]
"""

from __future__ import annotations

import argparse
import sqlite3
import sys
from datetime import date, timedelta
from pathlib import Path


def fmt_line(row: sqlite3.Row) -> str:
    status = "[x]" if row["is_done"] else "[ ]"
    parts = [f"- {status} {row['title']}"]
    if row["project"]:
        parts.append(f"@{row['project']}")
    if row["created"]:
        parts.append(f"+{row['created']}")
    if row["is_done"] and row["done"]:
        parts.append(f"*{row['done']}")
    parts.append(f"[{row['id']}]")
    return " ".join(parts)


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument(
        "--db",
        default=str(Path.home() / "Library/Application Support/Tick/data.db"),
    )
    ap.add_argument("--out", default=str(Path.home() / "hoard/.tick"))
    args = ap.parse_args()

    db_path = Path(args.db).expanduser()
    out_dir = Path(args.out).expanduser()
    out_dir.mkdir(parents=True, exist_ok=True)

    if not db_path.exists():
        print(f"db not found: {db_path}", file=sys.stderr)
        return 1

    conn = sqlite3.connect(str(db_path))
    conn.row_factory = sqlite3.Row
    rows = conn.execute(
        """
        SELECT
          f.id            AS id,
          f.title         AS title,
          COALESCE(p.name, '')             AS project,
          f.is_done       AS is_done,
          COALESCE(date(f.completed_at), '') AS done,
          date(f.created_at)               AS created
        FROM features f
        LEFT JOIN projects p ON f.project_id = p.id
        ORDER BY f.id ASC
        """
    ).fetchall()

    today = date.today()
    cutoff = today - timedelta(days=7)
    tasks_lines: list[str] = []
    archive_lines: list[str] = []

    for r in rows:
        line = fmt_line(r)
        if r["is_done"] and r["done"]:
            try:
                d = date.fromisoformat(r["done"])
                if d < cutoff:
                    archive_lines.append(line)
                    continue
            except ValueError:
                pass
        tasks_lines.append(line)

    tasks_path = out_dir / "tasks.md"
    archive_path = out_dir / "archive.md"
    tasks_path.write_text("\n".join(tasks_lines) + ("\n" if tasks_lines else ""))
    archive_path.write_text(
        "\n".join(archive_lines) + ("\n" if archive_lines else "")
    )

    print(f"wrote {tasks_path}: {len(tasks_lines)} lines")
    print(f"wrote {archive_path}: {len(archive_lines)} lines")
    return 0


if __name__ == "__main__":
    sys.exit(main())
