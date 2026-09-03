#!/usr/bin/env sh
# Runs the divergence measurement with the repository layout of this machine.
# The tool is its own Go module outside go.work, hence GOWORK=off.
# Any argument is passed through, e.g. ./run.sh -from 2026-08-01 -to 2026-08-31 -v
set -eu
cd "$(dirname "$0")"
GOWORK=off go run . "$@"
