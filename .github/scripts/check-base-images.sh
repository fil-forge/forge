#!/usr/bin/env bash
# Every external base image in a Dockerfile must be pinned by digest.
#
# Rule 2: build what we own, pin what we don't. #6 applied it to the images
# the stack *pulls* and stopped there, so the images our own Dockerfiles are
# *built from* kept floating -- golang:1.27-bookworm, alpine:latest,
# debian:bookworm-slim. A floating base makes an image a function of the
# registry as well as the commit, which is the same defect #6 existed to fix,
# one layer down.
#
# renovate.json asks Renovate to keep these current, but the Renovate app is
# not installed yet (MONOREPO_TODO.md), so that half is inert. This check is
# the half that works today: it does not bump anything, it only refuses to let
# a new unpinned base arrive unnoticed.
set -euo pipefail

cd "$(dirname "$0")/../.."

status=0
checked=0
for df in $(find . -name 'Dockerfile*' -not -path './.git/*' | sort); do
  # Stage names defined by `AS <name>`. A later FROM naming one of these is an
  # internal reference (hilt's `FROM build AS build-prod`), not an image.
  stages=$(grep -iE '^[[:space:]]*FROM[[:space:]]' "$df" |
           sed -nE 's/.*[[:space:]][Aa][Ss][[:space:]]+([^[:space:]]+).*/\1/p')

  while IFS= read -r line; do
    # Drop the FROM keyword and any --flags, then take the image reference.
    ref=$(printf '%s' "$line" |
          sed -E 's/^[[:space:]]*[Ff][Rr][Oo][Mm][[:space:]]+//; s/--[^[:space:]]+[[:space:]]+//g' |
          awk '{print $1}')

    [ "$ref" = "scratch" ] && continue
    if printf '%s\n' "$stages" | grep -qxF "$ref"; then
      continue
    fi

    checked=$((checked + 1))
    case "$ref" in
      *@sha256:*) echo "ok   $df: $ref" ;;
      *) echo "FAIL $df: $ref is not pinned by digest"; status=1 ;;
    esac
  done < <(grep -iE '^[[:space:]]*FROM[[:space:]]' "$df")
done

if [ "$status" -eq 0 ]; then
  echo "All $checked external base images are pinned by digest."
else
  echo
  echo "Pin it as tag@sha256:<index digest>, keeping the tag so the file still"
  echo "says what it runs. Resolve the *index* digest, not a per-architecture"
  echo "one, or cross-platform builds break:"
  echo "  docker buildx imagetools inspect <image>:<tag> | head -2"
fi
exit "$status"
