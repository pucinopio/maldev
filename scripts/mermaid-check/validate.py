#!/usr/bin/env python3
"""Extract every ```mermaid block from docs/ and render each with mmdc.

Exits 0 if all blocks render. Exits non-zero on any failure, printing the
file path, the diagram source, and the renderer stderr so the offending
diagram is easy to find.

Usage:
    python3 scripts/mermaid-check/validate.py [--mmdc=path] [--root=dir]

CI uses `npm install -g @mermaid-js/mermaid-cli` so `mmdc` is on PATH;
local users can either follow the same recipe or `npm install` in
scripts/mermaid-check (the .bin/mmdc shim works on Linux/macOS).
"""
from __future__ import annotations

import argparse
import pathlib
import re
import shutil
import subprocess
import sys
import tempfile

MERMAID_BLOCK = re.compile(r"```mermaid\n(.*?)```", re.DOTALL)


def find_blocks(root: pathlib.Path):
    for md in sorted(root.rglob("*.md")):
        text = md.read_text(encoding="utf-8")
        for i, m in enumerate(MERMAID_BLOCK.finditer(text), start=1):
            yield md, i, m.group(1)


def render(mmdc: str, source: str) -> tuple[bool, str]:
    with tempfile.TemporaryDirectory() as td:
        inp = pathlib.Path(td) / "in.mmd"
        out = pathlib.Path(td) / "out.svg"
        inp.write_text(source, encoding="utf-8")
        proc = subprocess.run(
            [mmdc, "-i", str(inp), "-o", str(out), "--quiet"],
            capture_output=True,
            text=True,
            timeout=120,
        )
        if proc.returncode != 0 or not out.exists():
            return False, (proc.stderr or proc.stdout).strip()
        return True, ""


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument(
        "--mmdc",
        default="",
        help="Path or name of the mmdc binary (default: search PATH or the local node_modules shim)",
    )
    ap.add_argument("--root", default="docs")
    args = ap.parse_args()

    mmdc = args.mmdc or shutil.which("mmdc") or "scripts/mermaid-check/node_modules/.bin/mmdc"
    if not (shutil.which(mmdc) or pathlib.Path(mmdc).exists()):
        print(f"FATAL: mmdc not found ({mmdc!r})", file=sys.stderr)
        print(
            "Install with `npm install -g @mermaid-js/mermaid-cli` or run "
            "`npm install` inside scripts/mermaid-check.",
            file=sys.stderr,
        )
        return 2

    root = pathlib.Path(args.root)
    if not root.is_dir():
        print(f"FATAL: root {args.root} is not a directory", file=sys.stderr)
        return 2

    failures: list[tuple[pathlib.Path, int, str, str]] = []
    total = 0
    for md, idx, src in find_blocks(root):
        total += 1
        ok, err = render(mmdc, src)
        if ok:
            print(f"  OK  {md} #{idx}")
        else:
            print(f"FAIL  {md} #{idx}", file=sys.stderr)
            failures.append((md, idx, src, err))

    print()
    print(f"Checked {total} mermaid block(s). Failures: {len(failures)}")
    if failures:
        print()
        for md, idx, src, err in failures:
            print(f"--- {md} block #{idx} ---")
            print("source:")
            for line in src.rstrip().splitlines():
                print(f"  | {line}")
            print("renderer output:")
            for line in err.splitlines()[:20]:
                print(f"  | {line}")
            print()
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
