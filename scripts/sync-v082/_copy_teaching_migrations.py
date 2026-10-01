"""One-shot helper: copy teaching migrations from main onto v0.8.2 with new numbers."""
from __future__ import annotations

import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]  # repo root via worktree → .worktrees → root? 
# This file lives at: <root>/.worktrees/sync-upstream-v0.8.2/scripts/sync-v082/
# parents[0]=sync-v082, [1]=scripts, [2]=worktree root, [3]=.worktrees — wrong.
# Prefer locating via git.
def repo_root() -> Path:
    out = subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True)
    return Path(out.strip())


def main_repo_root() -> Path:
    # worktree toplevel is the worktree; main files are fetched via git show main:
    return repo_root()


def git_show(ref_path: str) -> bytes:
    return subprocess.check_output(["git", "show", ref_path])


def main() -> int:
    wt = repo_root()
    versioned = [
        ("000091_tenant_default_agent", "000111_tenant_default_agent", "000091", "000111"),
        ("000097_tenant_notes", "000112_tenant_notes", "000097", "000112"),
        ("000098_announcements", "000113_announcements", "000098", "000113"),
        ("000094_platform_identity_flags", "000114_platform_identity_flags", "000094", "000114"),
        ("000095_workspace_ownership_anomalies", "000115_workspace_ownership_anomalies", "000095", "000115"),
        ("000096_single_active_share_link", "000116_single_active_share_link", "000096", "000116"),
    ]
    sqlite = [
        ("000016_tenant_default_agent", "000031_tenant_default_agent"),
        ("000017_tenant_notes", "000032_tenant_notes"),
        ("000018_announcements", "000033_announcements"),
        ("000013_platform_identity_flags", "000034_platform_identity_flags"),
        ("000014_workspace_ownership_anomalies", "000035_workspace_ownership_anomalies"),
        ("000015_single_active_share_link", "000036_single_active_share_link"),
    ]

    for old, new, oldn, newn in versioned:
        for d in ("up", "down"):
            data = git_show(f"main:migrations/versioned/{old}.{d}.sql")
            text = data.decode("utf-8")
            text = text.replace(f"Migration {oldn}", f"Migration {newn}")
            text = text.replace(f"Migration: {old[:6]}", f"Migration: {new[:6]}")
            out = wt / f"migrations/versioned/{new}.{d}.sql"
            out.write_text(text, encoding="utf-8", newline="\n")
            print(f"wrote {out.relative_to(wt)}")

    for old, new in sqlite:
        for d in ("up", "down"):
            data = git_show(f"main:migrations/sqlite/{old}.{d}.sql")
            out = wt / f"migrations/sqlite/{new}.{d}.sql"
            out.write_bytes(data)
            print(f"wrote {out.relative_to(wt)}")

    sample = (wt / "migrations/versioned/000111_tenant_default_agent.up.sql").read_text(
        encoding="utf-8"
    )
    if "\ufffd" in sample:
        print("ERROR: replacement char in 000111", file=sys.stderr)
        return 1
    print("sample:", sample.splitlines()[0])
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
