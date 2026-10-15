#!/usr/bin/env python3
"""Create a root-traced first batch of high-reachability uncovered targets."""

import argparse
import collections
import json
import xml.etree.ElementTree as ET
from pathlib import Path

XS = "{http://www.w3.org/2001/XMLSchema}"
PREFIXES = {
    "cac": "urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2",
    "cbc": "urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2",
    "ext": "urn:oasis:names:specification:ubl:schema:xsd:CommonExtensionComponents-2",
    "udt": "urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2",
    "qdt": "urn:oasis:names:specification:ubl:schema:xsd:QualifiedDataTypes-2",
    "ccts-cct": "urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2",
    "sac": "urn:oasis:names:specification:ubl:schema:xsd:SignatureAggregateComponents-2",
    "sbc": "urn:oasis:names:specification:ubl:schema:xsd:SignatureBasicComponents-2",
    "ds": "http://www.w3.org/2000/09/xmldsig#",
    "xades": "http://uri.etsi.org/01903/v1.3.2#",
}


def qname(value, namespace):
    if not value:
        return None
    if ":" not in value:
        return namespace, value
    prefix, name = value.split(":", 1)
    return PREFIXES.get(prefix), name


def local(tag):
    return tag.rsplit("}", 1)[-1]


class SchemaGraph:
    def __init__(self, xsd_dir):
        self.types = {}
        self.elements = {}
        self.roots = {}
        for path in sorted(xsd_dir.glob("**/*.xsd")):
            schema = ET.parse(path).getroot()
            ns = schema.get("targetNamespace", "")
            for node in schema:
                if local(node.tag) == "complexType" and node.get("name"):
                    self.types[(ns, node.get("name"))] = node
                elif local(node.tag) == "element" and node.get("name"):
                    self.elements[(ns, node.get("name"))] = node
            if "maindoc" in path.parts:
                for node in schema:
                    if local(node.tag) != "element" or not node.get("name"):
                        continue
                    root_type = qname(node.get("type"), ns)
                    if root_type in self.types:
                        self.roots[f"{node.get('name')} ({path.name})"] = (node.get("name"), root_type)

        self.edges = collections.defaultdict(list)
        self.global_refs = collections.defaultdict(set)
        for type_key, typ in self.types.items():
            ns = type_key[0]
            for node in typ.iter(XS + "element"):
                ref = qname(node.get("ref"), ns)
                if ref:
                    self.global_refs[type_key].add(ref)
                    decl = self.elements.get(ref)
                    dest = qname(decl.get("type"), ref[0]) if decl is not None else None
                    label = ref[1]
                else:
                    label = node.get("name")
                    dest = qname(node.get("type"), ns)
                if label and dest in self.types:
                    self.edges[type_key].append((label, dest))
            for node in typ.iter():
                if local(node.tag) in ("extension", "restriction"):
                    base = qname(node.get("base"), ns)
                    if base in self.types:
                        self.edges[type_key].append((f"(base {base[1]})", base))

    def trace(self, root_key, target):
        ns, _, kind, schema_path = target.split("|", 3)
        root_name, start = self.roots[root_key]
        if schema_path.startswith("global/"):
            wanted_element = (ns, schema_path.split("/", 2)[1])
            queue = collections.deque([(start, [root_name])])
            seen = {start}
            while queue:
                current, path = queue.popleft()
                if wanted_element in self.global_refs[current]:
                    return path + [wanted_element[1]]
                for label, dest in sorted(self.edges[current]):
                    if dest not in seen:
                        seen.add(dest)
                        queue.append((dest, path + [label]))
            return None
        owner = schema_path.split("/", 1)[0].split("/@", 1)[0]
        if kind in ("inheritance", "recursive"):
            owner = schema_path.split("->", 1)[0]
        owner_key = (ns, owner.removeprefix("inline/"))
        queue = collections.deque([(start, [root_name])])
        seen = {start}
        while queue:
            current, path = queue.popleft()
            if current == owner_key:
                suffix = schema_path[len(owner):].lstrip("/")
                return path + ([f"{owner} type"] if path[-1] != owner else []) + ([suffix] if suffix else [])
            for label, dest in sorted(self.edges[current]):
                if dest not in seen:
                    seen.add(dest)
                    queue.append((dest, path + [label]))
        return None


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("xsd_dir", type=Path)
    ap.add_argument("root_map_json", type=Path)
    ap.add_argument("--markdown", type=Path, required=True)
    ap.add_argument("--json", type=Path, required=True)
    ap.add_argument("--limit", type=int, default=30)
    args = ap.parse_args()

    root_map = json.loads(args.root_map_json.read_text())
    graph = SchemaGraph(args.xsd_dir)
    eligible = [t for t in root_map["targets"] if t["roots"]]
    eligible.sort(key=lambda t: (-len(t["roots"]), t["schemaFile"], t["schemaPath"], t["id"]))
    selected = eligible[:args.limit]

    traces_by_target = {}
    root_scores = collections.defaultdict(list)
    for target in selected:
        traces = {}
        for root in target["roots"]:
            path = graph.trace(root, target["id"])
            if path:
                traces[root] = path
                root_scores[root].append((target["id"], path))
        traces_by_target[target["id"]] = traces
    # Prefer one shared candidate root for the whole first batch. Maximize
    # targets traced, then minimize total path length, then use a stable name.
    shared_root = min(
        root_scores,
        key=lambda root: (-len(root_scores[root]),
                          sum(len(path) for _, path in root_scores[root]), root),
    ) if root_scores else ""

    records = []
    for target in selected:
        traces = traces_by_target[target["id"]]
        if shared_root in traces:
            candidate_root, path = shared_root, traces[shared_root]
        elif traces:
            candidate_root, path = min(traces.items(), key=lambda item: (len(item[1]), item[0]))
        else:
            candidate_root, path = target["roots"][0], []
        record = dict(target)
        record["candidateRoot"] = candidate_root
        record["elementPath"] = path
        record["currentStatus"] = "target remains uncovered after global element reference accounting was corrected"
        if target["kind"] == "complexType":
            record["generationGapHypothesis"] = "existing fixtures do not instantiate this reachable named type"
        elif target["kind"] == "inheritance":
            record["generationGapHypothesis"] = "existing fixtures do not instantiate the derived type, so its base edge is unobserved"
        elif target["kind"] == "recursive":
            record["generationGapHypothesis"] = "reachable recursive type edge needs an explicit finite-depth witness"
        else:
            record["generationGapHypothesis"] = "inspect the target's owning type and add an XSD-valid witness"
        records.append(record)

    output = {
        "sourceRootMap": str(args.root_map_json),
        "batchSize": len(records),
        "sharedCandidateRoot": shared_root,
        "targets": records,
    }
    args.json.parent.mkdir(parents=True, exist_ok=True)
    args.json.write_text(json.dumps(output, indent=2) + "\n")
    md = [
        "# First prioritized batch of reachable uncovered targets", "",
        f"Source dependency map: `{args.root_map_json}`. This batch contains {len(records)} "
        f"of {len(eligible)} residual targets that the XSD dependency graph maps to main-document roots.", "",
    ]
    if records:
        md += [
            (f"The shortest-path scoring found one shared candidate root for the entire batch: "
             f"`{shared_root}`." if shared_root else ""), "",
            "The paths below trace from a candidate document root through element uses to the "
            "target's owning type. Only targets still uncovered after global-reference accounting "
            "was corrected are included.", "",
        ]
    else:
        md += [
            "No residual targets mapped to a main-document root.", "",
        ]
    md += [
        "| Target | Reachable roots | Candidate root | Element path |", "|---|---:|---|---|"]
    for record in records:
        path = " → ".join(record["elementPath"]) if record["elementPath"] else "trace unavailable"
        target_label = record["schemaPath"]
        md.append(f"| `{target_label}` ({record['schemaFile']}) | {len(record['roots'])} | `{record['candidateRoot']}` | {path} |")
    md += ["", "## Target IDs", ""]
    for record in records:
        md.append(f"- `{record['id']}`")
    args.markdown.parent.mkdir(parents=True, exist_ok=True)
    args.markdown.write_text("\n".join(md) + "\n")
    print(f"Wrote {len(records)} prioritized targets to {args.markdown}")


if __name__ == "__main__":
    main()
