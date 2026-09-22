#!/usr/bin/env bash
# Every in-repo module must be referred to by its monorepo path.
#
# This exists because of the half of a subtree pull that nothing reports. A
# conflict is loud: upstream touched an import block we had rewritten, git
# stops, and you fix it. A file upstream ADDED has no conflict -- it merges
# cleanly and arrives carrying `github.com/fil-forge/<svc>/...`, which is the
# polyrepo path and does not resolve here. Nothing mentions it. It surfaces as
# a build failure later, if you are lucky, or as a stale module downloaded from
# the old repo, if you are not.
#
# Pulling piri to b42bdf7 added five such files in one merge. That is the rate
# to expect, so this is a check rather than a habit.
#
# There is no list of services in here: they are the top-level directories with
# a go.mod, which is the same thing go.work lists. Adding a service adds it to
# this check.
#
# Two spellings are wrong, and they are wrong in different ways:
#
#   github.com/fil-forge/<svc>          an import path; the module is
#                                       github.com/fil-forge/forge/<svc> now
#   https://github.com/fil-forge/<svc>  a repository URL; the repository is
#                                       github.com/fil-forge/forge, with no
#                                       service on the end (piri serves this
#                                       value out of GET /, so a wrong one is
#                                       a 404 with our name on it)
#
# WHAT IS CHECKED, EXACTLY, because "Go sources and go.mod" read as "everything
# that is not a doc" and three other file types were wrong in the tree when
# this was written:
#
#   checked      *.go, go.mod
#   NOT checked  Makefile, *.yaml, *.json, *.sh, go.sum
#
# Docs and deploy scripts are deliberately out: those repositories still exist
# and still serve the releases those documents describe. The rest are named
# rather than left implied -- .mockery.yaml and renovate.json already carry
# monorepo paths, and the dead -X ldflags in piri/, hilt/ and sprue/Makefile
# belong to #16, which adds the guard for that class.
set -euo pipefail

cd "$(dirname "$0")/../.."

svcs=$(for d in */; do [ -f "$d/go.mod" ] && printf '%s\n' "${d%/}"; done | paste -sd'|')
[ -n "$svcs" ] || { echo "no modules found -- is this the repository root?" >&2; exit 2; }

status=0

# `grep -o` then filter the MATCH, not the line. `grep -v` on the whole line
# dropped any line carrying a polyrepo path AND a monorepo path together --
# which is what an import block looks like mid-rewrite -- so the guard passed
# over exactly the file a half-finished rewrite produces. Verified on a fixture
# before and after.
#
# The `grep -v` matches nothing today: $svcs is the top-level directories with
# a go.mod and there is no `forge/` among them, so no match can end in /forge.
# It is here for the day one is added, when `forge\b` would otherwise match
# inside every correct monorepo path. Do not read it as the fix -- the fix is
# the -o above.
if hits=$(grep -rnoE "github\.com/fil-forge/($svcs)(\b|$)" \
            --include='*.go' --include='go.mod' . \
          | grep -v ':github\.com/fil-forge/forge$'); then
  echo "Polyrepo module paths in Go sources:"
  printf '%s\n' "$hits"
  echo
  echo "Rewrite each to github.com/fil-forge/forge/<svc>. If it arrived in a"
  echo "subtree pull, expect siblings: a cleanly-merged new file raises no"
  echo "conflict, so nothing else will have told you about it."
  status=1
fi

if hits=$(grep -rnE "https://github\.com/fil-forge/($svcs)\b" \
            --include='*.go' --include='go.mod' .); then
  echo "Polyrepo repository URLs in Go sources:"
  printf '%s\n' "$hits"
  echo
  echo "The repository is https://github.com/fil-forge/forge -- no service on"
  echo "the end. Services serve this value from GET /, so a wrong one ships."
  status=1
fi

# Name the scope in the success line, not only in the header. "All in-repo
# module references use their monorepo paths" is what a reviewer reads, and
# it was not true of the tree it printed in: piri/Makefile names
# github.com/fil-forge/piri/cmd as its build target, which is #16's. A guard
# over part of a chain reads exactly like a guard over the chain (rule 5), and
# that applies to what it says when it passes as much as to what it looks at.
[ "$status" -eq 0 ] &&
  echo "All *.go and go.mod in-repo module references use their monorepo paths."
[ "$status" -eq 0 ] &&
  echo "(Makefile, *.yaml, *.json, *.sh and go.sum are not checked -- see header.)"
exit "$status"
