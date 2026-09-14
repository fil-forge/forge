#!/usr/bin/env bash
# Retry a command that can fail on a transient proxy.golang.org error.
#
# proxy.golang.org intermittently drops an HTTP/2 stream mid-transfer:
#
#   read "https://proxy.golang.org/<mod>/@v/<ver>.zip":
#   stream error: stream ID 205; INTERNAL_ERROR; received from peer
#
# The go command does not retry these, so one dropped stream out of several
# hundred module fetches fails the whole step. Six occurrences in this
# repository so far, across four different modules and four different jobs.
#
# Use this ONLY for dependency resolution (go mod download, go mod tidy).
# Never wrap `go test`: a retry there would mask a flaky test, which is
# exactly the failure we most need to see.
set -uo pipefail

attempts=${RETRY_ATTEMPTS:-3}

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
