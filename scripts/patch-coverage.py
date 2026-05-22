#!/usr/bin/env python3
"""Compute patch coverage: percentage of added lines (from diff) covered by tests."""

import subprocess
import sys
import re
from collections import defaultdict
from pathlib import Path


def get_module_prefix() -> str:
    """Read module path from go.mod."""
    try:
        result = subprocess.run(
            ["go", "list", "-m"],
            capture_output=True, text=True, check=True,
        )
        return result.stdout.strip() + "/"
    except Exception:
        # Fallback: parse go.mod directly
        with open("go.mod") as f:
            for line in f:
                if line.startswith("module "):
                    return line.split(None, 1)[1].strip() + "/"
    return ""


def parse_diff_added_lines(base_ref: str) -> dict[str, set[int]]:
    """Return {file: set_of_added_line_numbers} from git diff against base_ref."""
    result = subprocess.run(
        ["git", "diff", f"origin/{base_ref}...HEAD", "--unified=0", "--no-color", "--diff-filter=ACMR"],
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        print(f"git diff failed: {result.stderr}", file=sys.stderr)
        sys.exit(1)

    added = defaultdict(set)
    current_file = None
    line_no = 0
    remaining = 0

    for line in result.stdout.splitlines():
        if line.startswith("+++ b/"):
            current_file = line[6:]
        elif line.startswith("@@"):
            match = re.search(r"\+(\d+)(?:,(\d+))?", line)
            if match:
                line_no = int(match.group(1))
                remaining = int(match.group(2) or 1)
        elif line.startswith("+") and not line.startswith("+++"):
            if current_file and remaining > 0:
                added[current_file].add(line_no)
            line_no += 1
            remaining -= 1
        elif not line.startswith("-"):
            line_no += 1
            remaining -= 1

    return dict(added)


def parse_coverprofile(path: str, module_prefix: str) -> dict[str, list[tuple[int, int, int, int]]]:
    """Return {repo_relative_file: [(start_line, end_line, num_stmts, covered_stmts), ...]}."""
    blocks = defaultdict(list)
    with open(path) as f:
        for line in f:
            line = line.strip()
            if line.startswith("mode:"):
                continue
            parts = line.split()
            if len(parts) < 3:
                continue
            file_range = parts[0]
            num_stmts = int(parts[-2])
            covered = int(parts[-1])

            colon_idx = file_range.rfind(":")
            if colon_idx == -1:
                continue
            filepath = file_range[:colon_idx]
            range_str = file_range[colon_idx + 1:]

            # Normalize to repo-relative path
            if module_prefix and filepath.startswith(module_prefix):
                filepath = filepath[len(module_prefix):]

            match = re.match(r"(\d+)\.\d+,(\d+)\.\d+", range_str)
            if not match:
                continue
            start_line = int(match.group(1))
            end_line = int(match.group(2))

            blocks[filepath].append((start_line, end_line, num_stmts, covered))

    return dict(blocks)


def line_covered(line: int, blocks: list[tuple[int, int, int, int]]) -> bool:
    """Check if a line falls within any covered block."""
    for start, end, num_stmts, covered in blocks:
        if start <= line <= end:
            return covered > 0
    return False


def main():
    if len(sys.argv) < 2:
        print("Usage: patch-coverage.py <coverprofile> [base_ref]", file=sys.stderr)
        sys.exit(1)

    coverprofile = sys.argv[1]
    base_ref = sys.argv[2] if len(sys.argv) > 2 else "main"

    if not Path(coverprofile).exists():
        print(f"Coverprofile not found: {coverprofile}", file=sys.stderr)
        sys.exit(1)

    module_prefix = get_module_prefix()
    added_lines = parse_diff_added_lines(base_ref)

    if not added_lines:
        print("No added lines found in diff — patch coverage: N/A")
        print("PATCH_COVERAGE=N/A")
        return

    coverage = parse_coverprofile(coverprofile, module_prefix)

    total_added = 0
    covered_added = 0
    uncovered_files = {}

    for filepath, lines in sorted(added_lines.items()):
        blocks = coverage.get(filepath, [])
        for ln in sorted(lines):
            total_added += 1
            if line_covered(ln, blocks):
                covered_added += 1
            else:
                uncovered_files.setdefault(filepath, []).append(ln)

    if total_added == 0:
        print("No added lines found in diff — patch coverage: N/A")
        print("PATCH_COVERAGE=N/A")
        return

    pct = covered_added / total_added * 100
    print(f"Patch coverage: {covered_added}/{total_added} lines covered ({pct:.1f}%)")

    if uncovered_files:
        print("\nUncovered lines:")
        for fp, lns in sorted(uncovered_files.items()):
            print(f"  {fp}: lines {', '.join(str(l) for l in lns)}")

    print(f"PATCH_COVERAGE={pct:.1f}")


if __name__ == "__main__":
    main()
