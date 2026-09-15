#!/usr/bin/env bash
# Run a command, retrying on failure with a widening delay.
#
#   retry.sh <command> [args...]
#   RETRY_ATTEMPTS=5 retry.sh <command> [args...]    # default 3
#
# Generic: it knows nothing about what it runs. Why a particular step needs
# retrying belongs in a comment at that step, not here.
#
# One rule about what to wrap: dependency resolution only (go mod download,
# go mod tidy). Never `go test` -- a retry there would mask a flaky test,
# which is the one failure we most need to see.
set -uo pipefail

attempts=${RETRY_ATTEMPTS:-3}

# Validate before use: `seq 1 0` and `seq 1 abc` both print nothing, so the
# loop below would never run and the script would fall off the end with
# status 0 -- reporting success for a command it never executed. That is the
# failure this script exists to prevent, one level up.
bad=""
case "$attempts" in
  ''|*[!0-9]*) bad=1 ;;
  *) [ "$attempts" -ge 1 ] || bad=1 ;;
esac
if [ -n "$bad" ]; then
  echo "::error::RETRY_ATTEMPTS must be a positive integer, got '$attempts'" >&2
  exit 2
fi

for i in $(seq 1 "$attempts"); do
  "$@" && exit 0
  status=$?
  if [ "$i" -eq "$attempts" ]; then
    echo "::error::'$*' failed after $attempts attempts"
    exit "$status"
  fi
  delay=$((i * 5))
  echo "::warning::'$*' failed (attempt $i/$attempts); retrying in ${delay}s"
  sleep "$delay"
done
