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
# Only Go sources and go.mod are checked. Docs and deploy scripts still point
# at the polyrepo URLs on purpose -- those repositories still exist and still
# serve the releases those documents describe.
set -euo pipefail

cd "$(dirname "$0")/../.."

svcs=$(for d in */; do [ -f "$d/go.mod" ] && printf '%s\n' "${d%/}"; done | paste -sd'|')
[ -n "$svcs" ] || { echo "no modules found -- is this the repository root?" >&2; exit 2; }

status=0

if hits=$(grep -rnE "github\.com/fil-forge/($svcs)\b" \
            --include='*.go' --include='go.mod' . \
          | grep -v 'github\.com/fil-forge/forge/'); then
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

[ "$status" -eq 0 ] && echo "All in-repo module references use their monorepo paths."
exit "$status"
