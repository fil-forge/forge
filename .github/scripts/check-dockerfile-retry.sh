#!/usr/bin/env bash
# Every `go mod download` in a Dockerfile must be retried.
#
# proxy.golang.org drops HTTP/2 streams mid-transfer (INTERNAL_ERROR) and the
# go command does not retry. CI-side commands go through
# .github/scripts/retry.sh, but that script is not inside the build context of
# services whose context is their own directory, so Dockerfiles retry inline.
#
# Three ways this has gone wrong, none caught by CI:
#   - a download left unretried (piri), which then failed a real build;
#   - one "handled" with `|| true`, which does not avoid the network, it just
#     defers the same fetch to `go build` where there is no retry;
#   - this check itself, whose first version tested whether the physical line
#     contained the substring `sleep`. That passed `RUN sleep 1 && go mod
#     download`, and rejected a correct retry written across continuation
#     lines -- which is how every other multi-part RUN in this repo is written.
#
# So: join continuations into one logical command, then require the download to
# appear more than once in it. A retry is repetition; that is the property.
set -euo pipefail

cd "$(dirname "$0")/../.."

python3 - <<'PY'
import pathlib, re, sys

status = 0
for path in sorted(pathlib.Path(".").rglob("Dockerfile*")):
    if ".git/" in str(path):
        continue

    # Join continuation lines, remembering where each logical line started.
    logical, buf, start = [], "", None
    for n, raw in enumerate(path.read_text().splitlines(), 1):
        if start is None:
            start = n
        buf += raw
        if raw.rstrip().endswith("\\"):
            buf = buf.rstrip()[:-1] + " "
            continue
        logical.append((start, buf))
        buf, start = "", None
    if buf:
        logical.append((start, buf))

    for lineno, cmd in logical:
        if cmd.lstrip().startswith("#") or "go mod download" not in cmd:
            continue
        # Strip trailing comments so a mention in prose does not count.
        code = cmd.split(" #", 1)[0]
        if not re.search(r"\bgo mod download\b", code):
            continue

        if re.search(r"\|\|\s*true", code):
            print(f"{path}:{lineno}: 'go mod download || true' hides the failure; "
                  f"retry instead", file=sys.stderr)
            status = 1
        elif len(re.findall(r"\bgo mod download\b", code)) < 2:
            print(f"{path}:{lineno}: 'go mod download' appears once, so it is "
                  f"not retried", file=sys.stderr)
            status = 1

if status:
    print(file=sys.stderr)
    print("A retry is the command repeated, e.g.:", file=sys.stderr)
    print("  (go mod download || (sleep 5 && go mod download) "
          "|| (sleep 15 && go mod download))", file=sys.stderr)
sys.exit(status)
PY
