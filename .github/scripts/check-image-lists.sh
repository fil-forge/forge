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
# There is a fourth link, and it is the one that actually broke: a test has to
# call stack.OptionsFromEnv() for any of the above to reach the stack at all.
# TestStackFromSnapshot did not, and ran the published images inside the job
# that existed to test HEAD. Nothing about that is visible in a workflow file.
#
# This check has twice been a guard over part of a chain: it first compared
# only images.yml against e2e.yml's build list, then only those two plus the
# env block. Each time it read exactly like a guard over the whole chain --
# same name, same green tick -- which is worse than no guard, because it is
# trusted. Both gaps were found in review, neither by CI.
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

# Link 4: a test that boots a stack must append OptionsFromEnv(), or none of
# the plumbing above has any effect. File-level check on purpose -- parsing Go
# to find the option list is more machinery than the property needs.
e2e_dir = pathlib.Path("smelt/tests/e2e")
booting = []
for f in sorted(e2e_dir.glob("*_test.go")):
    text = f.read_text()
    if re.search(r"\bstack\.(Must)?NewStack\(", text):
        booting.append(f)
        if "OptionsFromEnv" not in text:
            fail(f"{f} boots a stack without stack.OptionsFromEnv()",
                 "it would run the published images and pass.")
if not booting:
    fail("no test under smelt/tests/e2e boots a stack",
         "this check is not looking at what it thinks it is.")

for f in booting:
    print(f"ok   {f}: boots a stack and reads the environment")

for svc in sorted(matrix & built & set(tagged)):
    var = tagged[svc]
    if var in known:
        print(f"ok   {svc}: in matrix, built by e2e, passed as {var}")

if failed:
    sys.exit(1)

print(f"\nAll {len(matrix)} service images are built by both workflows, reach "
      f"the stack through a variable OptionsFromEnv reads, and every e2e test "
      f"that boots a stack reads it.")
PY
