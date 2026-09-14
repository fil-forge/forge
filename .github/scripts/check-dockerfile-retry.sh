#!/usr/bin/env bash
# Every `go mod download` in a Dockerfile must be retried.
#
# proxy.golang.org drops HTTP/2 streams mid-transfer (INTERNAL_ERROR) and the
# go command does not retry. CI-side commands go through
# .github/scripts/retry.sh, but that script is not inside the build context of
# services whose context is their own directory, so Dockerfiles retry inline.
#
# Two ways this has gone wrong already, both found by review rather than by a
# check: a download left unretried (piri, which then failed in CI), and one
# "handled" with `|| true`, which does not avoid the network -- it defers the
# same fetch to `go build`, where there is no retry and the error is harder to
# read.
#
# Worth asserting because new services keep arriving with their own Dockerfile.
set -euo pipefail

cd "$(dirname "$0")/../.."

status=0
while IFS= read -r f; do
  while IFS=: read -r lineno line; do
    # Skip comments.
    case "$(printf '%s' "$line" | sed 's/^[[:space:]]*//')" in '#'*) continue ;; esac

    if printf '%s' "$line" | grep -q '|| *true'; then
      echo "$f:$lineno: 'go mod download || true' hides the failure; retry instead" >&2
      status=1
    elif ! printf '%s' "$line" | grep -q 'sleep'; then
      echo "$f:$lineno: 'go mod download' is not retried" >&2
      status=1
    fi
  done < <(grep -n 'go mod download' "$f" || true)
done < <(find . -name 'Dockerfile*' -not -path './.git/*' | sort)

if [ "$status" -ne 0 ]; then
  echo >&2
  echo "Wrap it, e.g.:" >&2
  echo "  (go mod download || (sleep 5 && go mod download) || (sleep 15 && go mod download))" >&2
fi
exit "$status"
