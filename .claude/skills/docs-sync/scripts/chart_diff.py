#!/usr/bin/env python3
"""Summarize what a Helm PR changes in each camunda-platform chart line.

Compares the merge base of BASE and HEAD against HEAD and prints JSON:
per chart minor, which documented values were added, removed, possibly
renamed, or had their default or description changed, plus which other
chart files changed and whether the commits declare a breaking change.

It only reports facts from the diff. Deciding what the docs need is left
to the skill, which reads the actual diff for anything flagged here.

Usage:
  python chart_diff.py --base origin/main [--head HEAD] [--repo .]
  python chart_diff.py --base origin/main --docs-repo ../camunda-docs
"""
import argparse
import difflib
import json
import re
import subprocess
import sys
from pathlib import Path

try:
    import yaml
except ImportError:  # pragma: no cover
    sys.exit("PyYAML is required: pip install pyyaml")

CHART_RE = re.compile(r"^charts/camunda-platform-(\d+\.\d+)/(.+)$")
PARAM_RE = re.compile(r"^\s*##\s*@(param|extra)\s+(\S+)\s*(.*)$")
BREAKING_SUBJECT_RE = re.compile(r"^\w+(\([^)]*\))?!:")

# Files whose changes never need user docs on their own.
NOISE_PATTERNS = [
    r"^test/",
    r"^go\.(mod|sum)$",
    r"^CHANGELOG\.md$",
    r"^RELEASE-NOTES\.md$",
    r"^Chart\.lock$",
    r"^values-digest\.yaml$",
]


def git(repo, *args, check=True):
    res = subprocess.run(["git", "-C", repo, *args], capture_output=True, text=True)
    if check and res.returncode != 0:
        raise RuntimeError(f"git {' '.join(args)} failed: {res.stderr.strip()}")
    return res.stdout


def show(repo, ref, path):
    res = subprocess.run(["git", "-C", repo, "show", f"{ref}:{path}"], capture_output=True, text=True)
    return res.stdout if res.returncode == 0 else None


def parse_params(text):
    """Return {key: description} for every ## @param / ## @extra line."""
    params = {}
    for line in (text or "").splitlines():
        m = PARAM_RE.match(line)
        if m:
            params[m.group(2)] = m.group(3).strip()
    return params


def lookup(data, dotted):
    """Fetch a value by dotted path; returns (found, value)."""
    cur = data
    for part in dotted.split("."):
        if isinstance(cur, dict) and part in cur:
            cur = cur[part]
        else:
            return False, None
    return True, cur


def fmt(value):
    return json.dumps(value, sort_keys=True, default=str)


def values_diff(base_text, head_text):
    base_params, head_params = parse_params(base_text), parse_params(head_text)
    try:
        base_data = yaml.safe_load(base_text or "") or {}
        head_data = yaml.safe_load(head_text or "") or {}
    except yaml.YAMLError as exc:
        return {"error": f"could not parse values.yaml: {exc}"}

    added = sorted(set(head_params) - set(base_params))
    removed = sorted(set(base_params) - set(head_params))

    default_changed, description_changed = [], []
    for key in sorted(set(base_params) & set(head_params)):
        bf, bv = lookup(base_data, key)
        hf, hv = lookup(head_data, key)
        if (bf or hf) and fmt(bv) != fmt(hv):
            default_changed.append({"key": key, "old": bv, "new": hv})
        if base_params[key] != head_params[key]:
            description_changed.append({"key": key, "old": base_params[key], "new": head_params[key]})

    # Keep only the most specific changed keys: a change to console.image.tag
    # also shows up on console and console.image, which is just noise.
    changed_keys = {d["key"] for d in default_changed}
    default_changed = [d for d in default_changed
                       if not any(k.startswith(d["key"] + ".") for k in changed_keys)]
    # Image tag/digest bumps are routine (Renovate) and never need docs on their own.
    image_bumps = [d for d in default_changed if re.search(r"(^|\.)image\.(tag|digest)$", d["key"])]
    default_changed = [d for d in default_changed if d not in image_bumps]

    # Pair removed/added keys that look like the same setting under a new name.
    renames, used = [], set()
    for old in removed:
        best, best_score = None, 0.0
        for new in added:
            if new in used:
                continue
            score = difflib.SequenceMatcher(None, base_params[old], head_params[new]).ratio()
            if old.split(".")[-1] == new.split(".")[-1]:
                score = max(score, 0.8)
            if score > best_score:
                best, best_score = new, score
        if best and best_score >= 0.75:
            renames.append({"old": old, "new": best, "confidence": round(best_score, 2)})
            used.add(best)
    renamed_old = {r["old"] for r in renames}

    deprecated = [k for k in head_params
                  if "deprecat" in head_params[k].lower() and "deprecat" not in base_params.get(k, "").lower()]

    return {
        "added": [{"key": k, "description": head_params[k], "default": lookup(head_data, k)[1]}
                  for k in added if k not in used],
        "removed": [{"key": k, "description": base_params[k]} for k in removed if k not in renamed_old],
        "possible_renames": renames,
        "default_changed": default_changed,
        "image_bumps": [{"key": d["key"], "old": d["old"], "new": d["new"]} for d in image_bumps],
        "description_changed": description_changed,
        "newly_deprecated": sorted(deprecated),
    }


def breaking_signals(repo, base, head):
    log = git(repo, "log", "--format=%H%x1f%s%x1f%b%x1e", f"{base}..{head}")
    signals = []
    for entry in filter(None, (e.strip() for e in log.split("\x1e"))):
        sha, subject, body = (entry.split("\x1f") + ["", ""])[:3]
        if BREAKING_SUBJECT_RE.match(subject) or "BREAKING CHANGE" in body:
            signals.append({"commit": sha[:10], "subject": subject})
    return signals


def docs_target(minor, docs_repo):
    """Map a chart minor to its folder in camunda-docs, or explain why there is none."""
    root = Path(docs_repo)
    versions = json.loads((root / "versions.json").read_text())
    if minor in versions:
        folder = f"versioned_docs/version-{minor}"
        return {"folder": folder, "kind": "released"} if (root / folder).is_dir() else \
            {"folder": None, "reason": f"{folder} listed in versions.json but missing"}
    newest = max(versions, key=lambda v: tuple(map(int, v.split("."))))
    if tuple(map(int, minor.split("."))) > tuple(map(int, newest.split("."))):
        return {"folder": "docs", "kind": "next"}
    return {"folder": None, "reason": f"{minor} is no longer maintained in camunda-docs"}


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--base", required=True, help="Base ref, e.g. origin/main")
    ap.add_argument("--head", default="HEAD")
    ap.add_argument("--repo", default=".", help="Path to camunda-platform-helm")
    ap.add_argument("--docs-repo", help="Path to a camunda-docs checkout, to add version mapping")
    args = ap.parse_args()

    merge_base = git(args.repo, "merge-base", args.base, args.head).strip()
    changed = git(args.repo, "diff", "--name-only", merge_base, args.head).splitlines()

    charts = {}
    for path in changed:
        m = CHART_RE.match(path)
        if not m:
            continue
        minor, rel = m.groups()
        bucket = "noise" if any(re.search(p, rel) for p in NOISE_PATTERNS) else "files"
        charts.setdefault(minor, {"files": [], "noise": []})[bucket].append(rel)

    result = {"merge_base": merge_base[:10], "head": git(args.repo, "rev-parse", "--short=10", args.head).strip(),
              "breaking_signals": breaking_signals(args.repo, merge_base, args.head),
              "charts": {}}

    for minor in sorted(charts, key=lambda v: tuple(map(int, v.split(".")))):
        entry = charts[minor]
        values_path = f"charts/camunda-platform-{minor}/values.yaml"
        info = {"changed_files": sorted(entry["files"]), "ignored_files": sorted(entry["noise"])}
        if "values.yaml" in entry["files"]:
            info["values"] = values_diff(show(args.repo, merge_base, values_path),
                                         show(args.repo, args.head, values_path))
        info["templates_changed"] = any(f.startswith("templates/") for f in entry["files"])
        info["schema_changed"] = any(f.startswith("values.schema") for f in entry["files"])
        info["only_noise"] = not entry["files"]
        if args.docs_repo:
            info["docs_target"] = docs_target(minor, args.docs_repo)
        result["charts"][minor] = info

    print(json.dumps(result, indent=2, default=str))


if __name__ == "__main__":
    main()
