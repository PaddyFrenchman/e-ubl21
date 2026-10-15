#!/usr/bin/env python3
"""Classify distinct uncovered schema targets from createsamples coverage JSON."""

import argparse
import collections
import json
from pathlib import Path


def parse_target(target):
    parts = target.split("|", 3)
    if len(parts) != 4:
        raise ValueError(f"unexpected target ID: {target!r}")
    namespace, schema_file, kind, schema_path = parts
    return namespace, schema_file, kind, schema_path


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("coverage_json", type=Path)
    parser.add_argument("--markdown", type=Path, required=True)
    args = parser.parse_args()

    report = json.loads(args.coverage_json.read_text())
    categorized = collections.defaultdict(set)
    for category, targets in report["uncovered"].items():
        for target in targets:
            categorized[category].add(target)
    all_targets = set().union(*categorized.values()) if categorized else set()

    by_kind = collections.Counter(parse_target(t)[2] for t in all_targets)
    by_file = collections.Counter(parse_target(t)[1] for t in all_targets)
    by_file_kind = collections.Counter(
        (parse_target(t)[1], parse_target(t)[2]) for t in all_targets
    )

    out = [
        "# Distinct uncovered schema targets",
        "",
        f"Source report: `{args.coverage_json}` (schema version {report['schemaVersion']}; "
        f"{report['fixtures']:,} fixtures).",
        "",
        f"There are **{len(all_targets):,} distinct target IDs**. The category arrays contain "
        f"{sum(map(len, categorized.values())):,} entries because targets can belong to multiple "
        "coverage categories. Each target ID encodes `namespace | XSD file | kind | schema path`.",
        "",
        "## Counts by target kind",
        "",
        "| Kind | Distinct targets |",
        "|---|---:|",
    ]
    for kind, count in sorted(by_kind.items(), key=lambda kv: (-kv[1], kv[0])):
        out.append(f"| `{kind}` | {count:,} |")

    out += ["", "## Counts by schema file", "", "| XSD file | Distinct targets |", "|---|---:|"]
    for schema_file, count in sorted(by_file.items(), key=lambda kv: (-kv[1], kv[0])):
        out.append(f"| `{schema_file}` | {count:,} |")

    out += [
        "",
        "## Counts by schema file and target kind",
        "",
        "| XSD file | Kind | Distinct targets |",
        "|---|---|---:|",
    ]
    for (schema_file, kind), count in sorted(by_file_kind.items()):
        out.append(f"| `{schema_file}` | `{kind}` | {count:,} |")

    out += ["", "## Distinct target IDs by schema file and kind", ""]
    grouped = collections.defaultdict(list)
    for target in all_targets:
        _, schema_file, kind, _ = parse_target(target)
        grouped[(schema_file, kind)].append(target)
    for (schema_file, kind), targets in sorted(grouped.items()):
        out += [f"### `{schema_file}` — `{kind}`", ""]
        for target in sorted(targets):
            namespace, _, _, schema_path = parse_target(target)
            out.append(f"- `{target}`")
        out.append("")

    args.markdown.parent.mkdir(parents=True, exist_ok=True)
    args.markdown.write_text("\n".join(out).rstrip() + "\n")
    print(f"Wrote {len(all_targets)} distinct IDs to {args.markdown}")
    print(f"Category entries: {sum(map(len, categorized.values()))}")


if __name__ == "__main__":
    main()
