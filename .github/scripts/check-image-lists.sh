#!/usr/bin/env bash
# The chain from "images.yml builds it" to "the e2e stack runs it" must be
# unbroken, at every link.
#
# The inventory is stated three times, in three shapes: images.yml as a matrix
# (parallel, one job per service), e2e.yml as a sequential build list, and
# e2e.yml's job `env:` as *_IMAGE variables that pkg/stack.OptionsFromEnv
# reads. Every break fails silently in the same direction: the stack keeps
# booting, because compose falls back to the published
# ghcr.io/fil-forge/<svc>:main default, and the suite passes while testing an
# image that is not this commit's.
#
# That is the hazard the consolidation plan's Traps section calls silent
# green, so it gets a guard rather than a comment.
#
# This check originally compared only the first two, which left the third
# link -- by far the easiest to break, since the variable names do not match
# the service names -- unguarded. Deleting `UPLOAD_IMAGE` kept it green while
# the stack quietly ran published sprue. Found in review; the lesson is that a
# guard over part of a chain reads exactly like a guard over the chain.
set -euo pipefail

python3 - <<'PY'
import pathlib, re, sys, yaml

images = yaml.safe_load(pathlib.Path(".github/workflows/images.yml").read_text())
matrix = {row["service"] for row in images["jobs"]["build"]["strategy"]["matrix"]["include"]}

e2e_text = pathlib.Path(".github/workflows/e2e.yml").read_text()
built = set(re.findall(r"^\s+build\s+(\S+)\s", e2e_text, re.M))

# Link 3: every built tag must reach the stack through an *_IMAGE variable.
# Read the e2e job's env block for VAR: forge/<service>:head.
e2e = yaml.safe_load(e2e_text)
env = e2e["jobs"]["e2e"].get("env", {})
tagged = {}          # service -> variable that carries its :head tag
for var, value in env.items():
    m = re.fullmatch(r"forge/(\S+):head", str(value))
    if m:
        tagged[m.group(1)] = var

# ...and that variable must be one OptionsFromEnv actually reads. The names
# are not derivable from the service (sprue -> UPLOAD_IMAGE,
# piri-signing-service -> SIGNER_IMAGE), so read the table rather than guess.
opts = pathlib.Path("smelt/pkg/stack/options.go").read_text()
table = opts.split("envImageOptions = []struct", 1)[-1]
known = set(re.findall(r'\{"(\w+_IMAGE)",', table))
if not known:
    sys.exit("could not parse envImageOptions from smelt/pkg/stack/options.go")

failed = False

def fail(msg, detail):
    global failed
    print(f"\nFAIL: {msg}")
    print(f"      {detail}")
    failed = True

for svc in sorted(matrix - built):
    fail(f"{svc} is in images.yml's matrix but e2e.yml never builds it",
         "e2e would boot it as a published image and still pass.")
for svc in sorted(built - matrix):
    fail(f"{svc} is built by e2e.yml but absent from images.yml's matrix",
         "no standalone build check for it.")
for svc in sorted(built - set(tagged)):
    fail(f"{svc} is built by e2e.yml but no *_IMAGE variable carries its tag",
         "the stack would run the published image while the suite passed.")
for svc, var in sorted(tagged.items()):
    if var not in known:
        fail(f"{svc} is passed as {var}, which OptionsFromEnv does not read",
             f"known variables: {', '.join(sorted(known))}")

for svc in sorted(matrix & built & set(tagged)):
    var = tagged[svc]
    if var in known:
        print(f"ok   {svc}: in matrix, built by e2e, passed as {var}")

if failed:
    sys.exit(1)

print(f"\nAll {len(matrix)} service images are built by both workflows and "
      f"reach the stack through a variable OptionsFromEnv reads.")
PY
