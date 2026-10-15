#!/usr/bin/env python3
"""Map uncovered coverage IDs to UBL main-document roots through XSD refs."""

import argparse
import collections
import json
import re
import xml.etree.ElementTree as ET
from pathlib import Path

XS = "{http://www.w3.org/2001/XMLSchema}"
PARTICLES = {"sequence", "choice", "all"}


def local(tag):
    return tag.rsplit("}", 1)[-1]


class Schemas:
    def __init__(self, xsd_dir):
        self.docs = {}
        self.by_ns = {}
        self.types = {}
        self.elements = {}
        for path in sorted(xsd_dir.glob("**/*.xsd")):
            root = ET.parse(path).getroot()
            doc = {"file": path.name, "path": path, "ns": root.get("targetNamespace", ""), "root": root}
            self.docs[path.name] = doc
            self.by_ns[doc["ns"]] = doc
            for node in root:
                kind = local(node.tag)
                name = node.get("name")
                if kind == "complexType" and name:
                    self.types[(doc["ns"], name)] = (doc, node)
                elif kind == "element" and name:
                    self.elements[(doc["ns"], name)] = (doc, node)
        self.prefixes = {
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

    def qname(self, value, ns):
        if not value:
            return None
        if ":" not in value:
            return ns, value
        prefix, name = value.split(":", 1)
        return self.prefixes.get(prefix), name

    def type_edges(self, doc, typ):
        """Yield referenced element declarations and base types in a complex type."""
        for node in typ.iter():
            tag = local(node.tag)
            if tag == "element":
                ref = self.qname(node.get("ref"), doc["ns"])
                if ref and ref in self.elements:
                    yield ("element", ref)
                t = self.qname(node.get("type"), doc["ns"])
                if t and t in self.types:
                    yield ("type", t)
                inline = node.find(XS + "complexType")
                if inline is not None:
                    yield ("inline", (doc, inline))
            elif tag in ("extension", "restriction"):
                base = self.qname(node.get("base"), doc["ns"])
                if base and base in self.types:
                    yield ("type", base)

    def type_reachability(self):
        """For every global element/type, return transitively reachable type keys and elements."""
        cache = {}

        def walk_type(key, active):
            if key in active:
                return set(), set()
            if key in cache:
                return cache[key]
            doc, typ = self.types[key]
            types, elements = {key}, set()
            next_active = active | {key}
            for kind, value in self.type_edges(doc, typ):
                if kind == "element":
                    elements.add(value)
                    child_doc, child = self.elements[value]
                    t = self.qname(child.get("type"), child_doc["ns"])
                    if t in self.types:
                        ct, ce = walk_type(t, next_active)
                        types |= ct
                        elements |= ce
                    inline = child.find(XS + "complexType")
                    if inline is not None:
                        it, ie = walk_inline(child_doc, inline, next_active)
                        types |= it
                        elements |= ie
                elif kind == "type":
                    ct, ce = walk_type(value, next_active)
                    types |= ct
                    elements |= ce
                else:
                    it, ie = walk_inline(*value, next_active)
                    types |= it
                    elements |= ie
            cache[key] = (types, elements)
            return types, elements

        def walk_inline(doc, typ, active):
            types, elements = set(), set()
            for kind, value in self.type_edges(doc, typ):
                if kind == "element":
                    elements.add(value)
                    child_doc, child = self.elements[value]
                    t = self.qname(child.get("type"), child_doc["ns"])
                    if t in self.types:
                        ct, ce = walk_type(t, active)
                        types |= ct
                        elements |= ce
                elif kind == "type":
                    ct, ce = walk_type(value, active)
                    types |= ct
                    elements |= ce
                else:
                    it, ie = walk_inline(*value, active)
                    types |= it
                    elements |= ie
            return types, elements

        roots = {}
        for doc in self.docs.values():
            if "maindoc" not in doc["path"].parts:
                continue
            for node in doc["root"]:
                if local(node.tag) != "element" or not node.get("name"):
                    continue
                root_name = node.get("name")
                root_key = (doc["ns"], root_name)
                types, elements = set(), {root_key}
                t = self.qname(node.get("type"), doc["ns"])
                if t in self.types:
                    types, nested = walk_type(t, set())
                    elements |= nested
                inline = node.find(XS + "complexType")
                if inline is not None:
                    it, ie = walk_inline(doc, inline, set())
                    types |= it
                    elements |= ie
                roots[f"{root_name} ({doc['file']})"] = {"types": types, "elements": elements}
        return roots


def owner_of(kind, path):
    if path.startswith("global/"):
        return "global", path.split("/", 2)[1]
    if kind == "inheritance":
        return "type", path.split("->", 1)[0]
    if kind == "recursive":
        return "type", path.split("->", 1)[0]
    if kind == "complexType":
        return "type", path.removeprefix("inline/")
    return "type", path.split("/", 1)[0].split("/@", 1)[0]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("xsd_dir", type=Path)
    ap.add_argument("coverage_json", type=Path)
    ap.add_argument("--markdown", type=Path, required=True)
    ap.add_argument("--json", type=Path, required=True)
    args = ap.parse_args()

    schemas = Schemas(args.xsd_dir)
    roots = schemas.type_reachability()
    report = json.loads(args.coverage_json.read_text())
    targets = set(t for items in report["uncovered"].values() for t in items)
    mapped = []
    by_root = collections.defaultdict(list)
    unmapped = []
    for target in sorted(targets):
        ns, filename, kind, path = target.split("|", 3)
        owner_kind, owner_name = owner_of(kind, path)
        if owner_kind == "global":
            target_key = (ns, owner_name)
            reachable = [root for root, graph in roots.items() if target_key in graph["elements"]]
        else:
            target_key = (ns, owner_name)
            reachable = [root for root, graph in roots.items() if target_key in graph["types"]]
        record = {"id": target, "kind": kind, "schemaFile": filename, "namespace": ns,
                  "schemaPath": path, "owner": owner_name, "roots": sorted(reachable)}
        signature_schema = ("XAdES" in filename or "xmldsig" in filename or
                            "Signature" in filename or ns.endswith("01903/v1.3.2#") or
                            ns.endswith("01903/v1.4.1#") or ns.endswith("xmldsig#"))
        support = []
        if kind in ("choice", "choiceAlternative"):
            support.append("choice-branch")
        if kind == "any":
            support.append("wildcard-content")
        if kind == "recursive":
            support.append("recursive-fixture")
        if signature_schema:
            support.append("signature-vocabulary")
        record["supportFlags"] = support
        record["rootDisposition"] = (
            "reachable-from-existing-document-roots" if reachable else
            "signature-or-wildcard-dedicated-fixture-candidate" if signature_schema else
            "no-explicit-main-root-dependency-review-needed"
        )
        if not reachable:
            unmapped.append(record)
        for root in reachable:
            by_root[root].append(target)
        mapped.append(record)

    out = {
        "sourceCoverage": str(args.coverage_json),
        "fixtureCount": report["fixtures"],
        "distinctUncoveredTargets": len(targets),
        "mainDocumentRoots": len(roots),
        "mappedTargetCount": sum(bool(t["roots"]) for t in mapped),
        "unreachableTargetCount": len(unmapped),
        "roots": {root: sorted(ids) for root, ids in sorted(by_root.items())},
        "targets": mapped,
        "unreachableTargets": unmapped,
    }
    md = ["# Uncovered targets grouped by reachable document roots", "",
          f"Source: `{args.coverage_json}`; {len(targets):,} distinct targets from "
          f"{report['fixtures']:,} fixtures; {len(roots)} main-document roots.", "",
          "Reachability follows XSD element references, declared types, inheritance, nested particles, and inline complex types. A target is assigned to every root that can reach its owning element or type. This is structural schema reachability; a valid value may still require generator support for choices, wildcards, recursive edges, or signature vocabularies.", "",
          f"Targets reachable from at least one document root: **{out['mappedTargetCount']:,}**. "
          f"Targets without a mapped document root: **{len(unmapped):,}**.", "",
          "## Root groups", "",
          "| Document root | Reachable uncovered targets |", "|---|---:|"]
    for root, ids in sorted(by_root.items(), key=lambda item: (-len(item[1]), item[0])):
        md.append(f"| `{root}` | {len(set(ids)):,} |")
    md += ["", "## Shared target groups", "",
           "Targets reachable from many roots are good candidates for a small number of richer shared fixtures. Counts below are numbers of roots that can reach the target.", "",
           "| Reachable roots | Targets |", "|---:|---:|"]
    root_counts = collections.Counter(len(t["roots"]) for t in mapped)
    for n, count in sorted(root_counts.items(), reverse=True):
        md.append(f"| {n} | {count:,} |")
    remaining = {t["id"] for t in mapped if t["roots"]}
    portfolio = []
    while remaining:
        best_root, covered = max(
            ((root, remaining.intersection(ids)) for root, ids in by_root.items()),
            key=lambda item: (len(item[1]), item[0]),
        )
        if not covered:
            break
        portfolio.append({"root": best_root, "newTargets": len(covered), "targets": sorted(covered)})
        remaining -= covered
    out["greedyRootPortfolio"] = [
        {"root": item["root"], "newTargets": item["newTargets"]} for item in portfolio
    ]
    md += ["", "## Greedy root portfolio for reachable targets", "",
           "This is a planning heuristic: repeatedly select the root that reaches the most currently uncovered targets. It identifies useful root families, but it does not prove that one generated fixture per selected root will cover those targets or be XSD-valid.", "",
           "| Selected root | Newly reachable targets |", "|---|---:|"]
    for item in portfolio:
        md.append(f"| `{item['root']}` | {item['newTargets']:,} |")
    args.json.parent.mkdir(parents=True, exist_ok=True)
    args.json.write_text(json.dumps(out, indent=2) + "\n")
    exact_groups = collections.defaultdict(list)
    for record in mapped:
        exact_groups[tuple(record["roots"])].append(record)
    md += ["", "## Target clusters with identical root reachability", "",
           "Targets in each cluster share the same set of reachable roots, so they can be considered together when selecting a minimal fixture portfolio.", "",
           "| Reachable root set | Targets |", "|---|---:|"]
    for root_set, records in sorted(exact_groups.items(), key=lambda item: (-len(item[1]), item[0])):
        names = ", ".join(root_set) if root_set else "no main-document root"
        if len(names) > 180:
            names = f"{len(root_set)} roots (see JSON target map)"
        md.append(f"| {names} | {len(records):,} |")
    md += ["", "## Unreachable targets", ""]
    if unmapped:
        md += ["These targets are not reached from any main document root by explicit element/type references. Signature vocabulary is expected to need a signature fixture strategy because UBL admits it through extension/wildcard boundaries. Other declarations need review as standalone component roots or unused declarations before deciding they require a dedicated fixture.", ""]
        for record in unmapped:
            md.append(f"- `{record['id']}`")
    else:
        md.append("None.")
    md += ["", "## Full target-to-root map", "",
           "Each entry lists every reachable main-document root. The machine-readable version is available in the adjacent JSON artifact.", ""]
    for record in mapped:
        roots_for_target = record["roots"]
        label = ", ".join(f"`{r}`" for r in roots_for_target) if roots_for_target else "**no root**"
        flags = f" ({', '.join(record['supportFlags'])})" if record["supportFlags"] else ""
        md.append(f"- `{record['id']}` — {label}{flags}")
    args.markdown.parent.mkdir(parents=True, exist_ok=True)
    args.markdown.write_text("\n".join(md) + "\n")
    print(f"Mapped {len(targets)} distinct targets; reachable={out['mappedTargetCount']}; "
          f"unreachable={len(unmapped)}; roots={len(roots)}")


if __name__ == "__main__":
    main()
