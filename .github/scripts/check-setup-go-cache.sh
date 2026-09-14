#!/usr/bin/env bash
# Every actions/setup-go step must set cache-dependency-path.
#
# setup-go defaults to cache: true, but resolves go.sum from the repository
# root. This repo has no root go.sum -- each module carries its own -- so a
# step without an explicit path logs
#
#   Restore cache failed: Dependencies file is not found ...
#
# as a WARNING and then runs with no module cache whatsoever. Nothing fails and
# nothing is annotated, so only an explicit check catches it.
#
# Parses the YAML rather than scanning text. The first version of this script
# grepped for `uses: actions/setup-go@` and then looked ahead for the setting,
# stopping at the next line matching `^      - `. Both halves were wrong: a
# quoted `uses: 'actions/setup-go@v5'` was invisible, and a `steps:` list
# indented at four spaces ran the look-ahead past the end of the step, where it
# was satisfied by a different job's setting later in the file. Reading the
# structure removes the guessing.
set -euo pipefail

cd "$(dirname "$0")/../.."

python3 - <<'PY'
import pathlib, sys, yaml

status, seen = 0, 0
for wf in sorted(pathlib.Path(".github/workflows").glob("*.yml")):
    doc = yaml.safe_load(wf.read_text()) or {}
    for job_name, job in (doc.get("jobs") or {}).items():
        for i, step in enumerate(job.get("steps") or []):
            if not isinstance(step, dict):
                continue
            uses = str(step.get("uses") or "")
            if not uses.startswith("actions/setup-go@"):
                continue
            seen += 1
            with_ = step.get("with") or {}
            # An explicit `cache: false` is a deliberate opt-out; nothing to do.
            if with_.get("cache") is False:
                continue
            if not with_.get("cache-dependency-path"):
                label = step.get("name") or uses
                print(f"{wf}: job '{job_name}' step {i + 1} ({label}): "
                      f"setup-go without cache-dependency-path", file=sys.stderr)
                status = 1

# A scan that finds nothing must not report success: this repository always has
# setup-go steps, so zero hits means the scan broke, not that the tree is clean.
if seen == 0:
    print("no actions/setup-go steps found in .github/workflows -- "
          "this check is not looking at what it thinks it is", file=sys.stderr)
    sys.exit(1)

if status:
    print(file=sys.stderr)
    print("Add 'cache-dependency-path: <module>/go.sum' to each step above, "
          "or 'cache: false' to opt out deliberately.", file=sys.stderr)
else:
    print(f"All {seen} setup-go steps set cache-dependency-path.")
sys.exit(status)
PY
