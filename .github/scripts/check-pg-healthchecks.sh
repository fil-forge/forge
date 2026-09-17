#!/usr/bin/env bash
# Every pg_isready must name a host, so the healthcheck probes TCP.
#
# Without -h, pg_isready uses the Unix socket, and the official postgres image
# runs initdb against a temporary server started with `listen_addresses=''` --
# socket up, TCP refused. The healthcheck therefore passes *during* init, and a
# dependent with `condition: service_healthy` starts into a window where the
# port it actually connects to does not exist yet.
#
# This is not theoretical. From e2e run 35247949580, plc-postgres's own log
# against plc's crash:
#
#   16:55:27.593  temp server: listening on Unix socket ONLY
#   16:55:27.633  temp server: ready to accept connections   <- healthcheck goes green
#   16:55:29.441  temp server: shut down
#   16:55:29.887  real server: listening on IPv4 0.0.0.0:5432
#   16:55:30.531  plc exits: ECONNREFUSED 172.18.0.6:5432
#
# The healthcheck was green 2.25 seconds before TCP existed. That is why
# TestUploadAndRetrieve/filesystem failed 2 of 40 e2e runs naming a *different*
# container each time -- whichever dependent lost the race that run.
#
# The rule has no list in it. Every pg_isready is either given a host or it is
# a bug, so nothing here needs updating when a service is added. It covers Go
# as well as YAML because smelt/pkg/generate builds one of these strings.
set -euo pipefail

cd "$(dirname "$0")/../.."

status=0
checked=0
while IFS= read -r hit; do
  file="${hit%%:*}"
  rest="${hit#*:}"
  line="${rest%%:*}"
  checked=$((checked + 1))
  case "$rest" in
    *pg_isready*-h[[:space:]]*|*pg_isready*--host*) echo "ok   $file:$line" ;;
    *) echo "FAIL $file:$line pg_isready with no -h probes the Unix socket"; status=1 ;;
  esac
done < <(grep -rn 'pg_isready' --include='*.yml' --include='*.yaml' --include='*.go' . | grep -v '^\./\.git/')

if [ "$checked" -eq 0 ]; then
  echo "No pg_isready healthchecks found -- this check has nothing to guard."
  echo "That is suspicious rather than fine: it used to find several."
  exit 1
fi

if [ "$status" -eq 0 ]; then
  echo "All $checked pg_isready probes name a host."
else
  echo
  echo "Add -h 127.0.0.1. Without it the probe uses the Unix socket, which is"
  echo "up during initdb while TCP is not, so dependents start too early."
fi
exit "$status"
