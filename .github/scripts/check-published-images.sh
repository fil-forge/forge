#!/usr/bin/env bash
# The published image set is stated in three places. They must agree.
#
# smelt's compose files require these images (`${HILT_IMAGE:?...}`) rather than
# defaulting them, because a silent default meant a caller who forgot an
# override booted a published image and never heard about it. That leaves two
# places to supply them: `.env.published` for `make up`, and `publishedImages`
# in pkg/stack/options.go for Go callers of `stack.WithPublishedImages()`. The
# Makefile shells out to `docker compose` directly, so the Go table cannot
# reach it and the duplication is unavoidable.
#
# `.env.published` used to say that duplication could not drift silently,
# because a missing entry fails at `compose up` naming the variable. It failed
# at `compose up` and it drifted anyway: indexing-service was added to the Go
# table and not to the env file, so `make up` had been broken on main for as
# long as the indexer had been in the stack. Nothing in CI runs `make up`, so
# "not silent" and "not noticed" turned out to be the same thing.
#
# This is the check that makes the claim true. Nothing here is a list: all
# three sides are derived, so a service added or renamed needs no edit.
#
# NOT COVERED, and deliberately: images the stack runs that this repository
# does NOT build -- guppy, ipni, plc, blockchain, minio. Those keep their `:-`
# compose defaults and are pinned by digest instead, which is
# check-stack-images.sh's job. This check is only about the required ones.
set -euo pipefail

cd "$(dirname "$0")/../.."

fail() { echo "::error::$*" >&2; status=1; }
status=0

# 1. REQUIRED: every `${X_IMAGE:?...}` compose interpolation. Two sources, and
#    both are needed -- PIRI_IMAGE appears in no compose file at all, because
#    pkg/generate/compose.go emits piri's service definition. A glob over
#    systems/ alone reports seven of eight and looks complete.
#
#    EVERY extraction below ends in `|| true` and is then tested for
#    emptiness. Not a mask: under `set -o pipefail` a grep that matches
#    nothing kills the script mid-way, and an early exit at the second
#    extraction is indistinguishable from the first check having reported
#    and finished. That is exactly how the first draft of this file shipped
#    a second check that never ran.
#
#    RESTRICTED TO compose.yml, not all of systems/. Shell scripts that run
#    INSIDE the containers use the same `${X:?}` form for values compose never
#    supplies -- BAO_ADDR and INGOT_OPENBAO_TOKEN in
#    systems/ingot/openbao/init.sh, PIRI_POSTGRES_DATABASES in
#    systems/piri/postgres-init.sh. Those come from the container environment
#    and have nothing to do with `make up`'s interpolation. A recursive grep
#    happens to miss them today only because none is named *_IMAGE, which is
#    not a property worth relying on.
required=$(
  { find smelt/systems -name compose.yml -exec grep -hoE '\$\{[A-Z_]+_IMAGE:\?' {} + 2>/dev/null || true
    grep -rhoE '\$\{[A-Z_]+_IMAGE:\?' smelt/pkg/generate/ 2>/dev/null || true
  } | sed -E 's/\$\{([A-Z_]+):\?/\1/' | sort -u
)
if [ -z "$required" ]; then
  echo "::error::found no required \${X_IMAGE:?} interpolations at all." \
       "Refusing to report that as agreement." >&2
  exit 2
fi

# 2. PROVIDED: what .env.published sets.
provided=$(grep -oE '^[A-Z_]+_IMAGE=' smelt/.env.published 2>/dev/null | tr -d '=' | sort -u || true)

if [ "$required" != "$provided" ]; then
  fail "smelt/.env.published does not match the required compose interpolations."
  diff <(printf '%s\n' "$required") <(printf '%s\n' "$provided") \
    --label 'required by compose' --label 'set in .env.published' -u >&2 || true
fi

# 3. The two copies of the reference list must name the same images. Compared
#    as refs rather than by variable name, because the Go table keys on the
#    reference and a config field, not on the compose variable.
env_refs=$(grep -oE '^[A-Z_]+_IMAGE=.*' smelt/.env.published 2>/dev/null | cut -d= -f2- | sort -u || true)
# `/^}$/`, not `/^}/`: the declaration is `[]struct {` ... `}{` ... `}`, so a
# range ending at any line starting with `}` stops at the struct's own closing
# brace and never reaches a single entry. It read as an empty table.
go_refs=$(awk '/^var publishedImages = /,/^}$/' smelt/pkg/stack/options.go 2>/dev/null \
          | grep -oE '"[^"]+/[^"]+:[^"]+"' | tr -d '"' | sort -u || true)

if [ -z "$go_refs" ] || [ -z "$env_refs" ]; then
  echo "::error::could not read one of the two image lists" \
       "(publishedImages in smelt/pkg/stack/options.go, or smelt/.env.published)." \
       "Refusing to report that as agreement." >&2
  exit 2
fi

if [ "$env_refs" != "$go_refs" ]; then
  fail "smelt/.env.published and pkg/stack/options.go name different images."
  diff <(printf '%s\n' "$env_refs") <(printf '%s\n' "$go_refs") \
    --label '.env.published' --label 'publishedImages' -u >&2 || true
fi

if [ "$status" -eq 0 ]; then
  n=$(printf '%s\n' "$required" | grep -c .)
  echo "check-published-images: $n required images, agreed across compose," \
       ".env.published and publishedImages."
fi
exit "$status"
