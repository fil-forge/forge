#!/usr/bin/env bash
# images.yml and e2e.yml must build the same set of service images.
#
# They necessarily express it differently -- images.yml as a matrix (parallel,
# one job per service), e2e.yml as a sequential build list feeding *_IMAGE --
# so the inventory is stated twice. A service added to one and not the other
# fails silently in the worse direction: e2e keeps booting, because compose
# falls back to the published ghcr.io/fil-forge/<svc>:main default, and the
# suite passes while testing an image that is not this commit's.
#
# That is the hazard the consolidation plan's Traps section calls silent
# green, so it gets a guard rather than a comment.
set -euo pipefail

python3 - <<'PY'
import pathlib, re, sys, yaml

images = yaml.safe_load(pathlib.Path(".github/workflows/images.yml").read_text())
matrix = {row["service"] for row in images["jobs"]["build"]["strategy"]["matrix"]["include"]}

e2e = pathlib.Path(".github/workflows/e2e.yml").read_text()
built = set(re.findall(r"^\s+build\s+(\S+)\s", e2e, re.M))

missing_from_e2e = sorted(matrix - built)
missing_from_images = sorted(built - matrix)

for svc in sorted(matrix & built):
    print(f"ok   {svc}: built by both")

if missing_from_e2e:
    print(f"\nFAIL: in images.yml but not built by e2e.yml: {', '.join(missing_from_e2e)}")
    print("      e2e would boot these as published images and still pass.")
if missing_from_images:
    print(f"\nFAIL: built by e2e.yml but not in images.yml's matrix: "
          f"{', '.join(missing_from_images)}")
    print("      no standalone build check for these.")
if missing_from_e2e or missing_from_images:
    sys.exit(1)

print(f"\nBoth workflows build the same {len(matrix)} service images.")
PY
