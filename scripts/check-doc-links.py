#!/usr/bin/env python3
"""Strict broken-link checker for docs/*.md.

Scans every relative .md→.md link under docs/ and verifies the target
exists on disk. Exits non-zero with a per-link report on failure.

Used by .github/workflows/build.yml as a CI gate so a PR that breaks a
doc cross-link can't merge.

Counts only on-disk markdown targets — `http(s)://` and pure anchor
links (`#section`) are ignored. Anchor-tail on a .md link (`page.md#foo`)
is stripped before the file-exists check.
"""

import re
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
DOCS = REPO_ROOT / "docs"

LINK_RE = re.compile(r"\[[^\]]*?\]\(([^)#]+\.md)(#[^)]*)?\)")


def main() -> int:
    if not DOCS.exists():
        print(f"docs/ not found at {DOCS}", file=sys.stderr)
        return 2

    total = 0
    broken: list[tuple[str, str]] = []
    for md in DOCS.rglob("*.md"):
        text = md.read_text(encoding="utf-8", errors="replace")
        for m in LINK_RE.finditer(text):
            target = m.group(1)
            total += 1
            if target.startswith(("http://", "https://", "mailto:")):
                continue
            resolved = (md.parent / target).resolve()
            if not resolved.exists():
                broken.append((str(md.relative_to(REPO_ROOT)), target))

    print(f"checked {total} markdown→markdown links across docs/")
    if not broken:
        print("OK: all links resolve")
        return 0

    print(f"FAIL: {len(broken)} broken link(s):", file=sys.stderr)
    for src, target in broken:
        print(f"  {src} -> {target}", file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main())
