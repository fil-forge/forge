#!/usr/bin/env bash
# Every image a compose file names must be pinned by digest, or be a variable.
#
# #6 pinned the 18 images the stack pulled at the time. It could not pin what
# had not arrived yet: swarf came in on #3 and indexing-service on #10, each
# carrying its own compose and testcontainers references, and nothing noticed.
# That is the shape this check exists to stop -- not the references #6 fixed,
# but the next one to walk in.
#
# The rule has no list in it, deliberately. An `image:` value is either a
# ${VARIABLE} -- which smelt resolves at run time from pkg/stack, and which is
# how in-repo services are deliberately left unpinned -- or it is something we
# pull, and then it needs a digest. Nothing here has to be updated when a
# service is added.
#
# NOT COVERED: image references in Go (testcontainers). That population cannot
# be found by pattern: a string shaped like an image reference matches 367
# times in this repository, almost all of them `s3:GetObject`-style IAM
# actions and host:port pairs, while a rule narrow enough to avoid those
# misses real ones (indexing-service's valkey and minio, both found by hand).
# A guard over part of a class reads exactly like a guard over the class, so
# there is deliberately none here rather than a partial one.
set -euo pipefail

cd "$(dirname "$0")/../.."

status=0
checked=0
while IFS= read -r hit; do
  file="${hit%%:*}"
  value=$(printf '%s' "${hit#*:}" | sed -E 's/^[0-9]+:[[:space:]]*image:[[:space:]]*//; s/[[:space:]]*$//')

  # A variable is the deliberate escape hatch: smelt fills it from pkg/stack,
  # which is where the published-vs-built decision actually lives.
  case "$value" in
    \$\{*|\'\$\{*|\"\$\{*) continue ;;
  esac

  checked=$((checked + 1))
  case "$value" in
    *@sha256:*) echo "ok   $file: $value" ;;
    *) echo "FAIL $file: $value is not pinned by digest"; status=1 ;;
  esac
done < <(grep -rn '^[[:space:]]*image:[[:space:]]' --include='*.yml' --include='*.yaml' . | grep -v '^\./\.git/')

if [ "$status" -eq 0 ]; then
  echo "All $checked pulled compose images are pinned by digest."
else
  echo
  echo "Pin it as tag@sha256:<index digest>, keeping the tag. If it is an"
  echo "in-repo service the stack builds, it belongs in pkg/stack and the"
  echo "compose entry should be \${SERVICE_IMAGE:?...} instead."
fi
exit "$status"
