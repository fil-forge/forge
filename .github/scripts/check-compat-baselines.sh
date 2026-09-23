#!/usr/bin/env bash
# Run compat-baselines.sh against a stub registry, both directions.
#
# WHY THIS EXISTS. compat-baselines.sh decides what the compat suite pins
# against, and every interesting thing it does is a REFUSAL -- it exists to
# tell "this service has cut no release" apart from "the registry could not be
# read", because conflating them is what made the gate green while testing
# nothing. Those paths do not run in an ordinary compat run: a healthy registry
# takes the success branch every time. Without this, the refusals are prose.
#
# AGENTS.md: "A guard that passes proves nothing until you have seen it fail on
# the defect it is for." Each negative test matches its OWN message, because
# asserting `exit != 0` alone cannot tell "refused for the reason under test"
# from "failed for some other reason" -- and here every refusal exits 1, so
# that distinction is the only thing separating them.
#
# THE REGISTRY IS A STUB, not ghcr.io. A guard that reaches the network is a
# guard that goes red when GitHub has a bad afternoon, and it could not
# reproduce a 500 at the token endpoint even then. The stub answers whatever
# the case under test needs, keyed by the service name in the request.
#
# THE SERVICE LIST IS A FIXTURE TOO. compat-baselines.sh derives it from
# smelt/pkg/stack/options.go, so each case builds a tree with its own
# one-service options.go and runs the script from inside it. That also covers
# the derivation itself: a case with no match must exit 2 rather than resolve
# an empty fleet.
#
# VERIFIED BOTH DIRECTIONS, by deleting each guard from the script under test
# and watching this go red:
#
#   the 403 -> "no package" exit               -> FAIL (test 4)
#   the non-403 -> "could not read" exit       -> FAIL (test 5)
#   the `|| exit 1` on digest_for              -> FAIL (test 6)
#   the Docker-Content-Digest presence check   -> FAIL (test 7)
#   the digest length check                    -> FAIL (test 8)
#   the empty-service-list refusal             -> FAIL (test 9)
#   numeric sorting in versions_from           -> FAIL (test 3)
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
under_test="$here/compat-baselines.sh"
[ -x "$under_test" ] || { echo "not executable: $under_test" >&2; exit 2; }

work=$(mktemp -d)
trap 'kill "${stub_pid:-}" 2>/dev/null || true; rm -rf "$work"' EXIT

fails=0
ok()   { printf '  ok    %s\n' "$1"; }
bad()  { printf '  FAIL  %s\n' "$1" >&2; fails=$((fails + 1)); }

# The stub. One service name per behaviour, so a case selects its branch by
# naming the service it asks about.
cat > "$work/stub.py" <<'PY'
import re, sys
from http.server import BaseHTTPRequestHandler, HTTPServer

DIGEST = "sha256:" + "a" * 64

TAGS = {
    "rel":     ["main", "sha-abc1234", "1.2.3"],
    "sorting": ["1.2.3", "1.10.0", "0.9.9", "main"],
    "flo":     ["main", "sha-abc1234"],
    "nomain":  ["sha-abc1234"],
    "nodig":   ["main"],
    "shortdig": ["main"],
}

class H(BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass

    def _svc(self):
        m = re.search(r"repository:fil-forge/([^:]+):pull", self.path)
        if m:
            return m.group(1)
        m = re.search(r"/v2/fil-forge/([^/]+)/", self.path)
        return m.group(1) if m else ""

    def _send(self, code, body=b"", headers=()):
        self.send_response(code)
        for k, v in headers:
            self.send_header(k, v)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        if body and self.command != "HEAD":
            self.wfile.write(body)

    def do_GET(self):
        svc = self._svc()
        if self.path.startswith("/token"):
            if svc == "gone":
                return self._send(403, b'{"errors":[{"code":"DENIED"}]}')
            if svc == "broken":
                return self._send(500, b"upstream is having a day")
            return self._send(200, b'{"token":"stub"}')
        if "/manifests/" in self.path:
            return self._manifest(svc)
        if "/tags/list" in self.path:
            import json
            tags = TAGS.get(svc, [])
            return self._send(200, json.dumps({"tags": tags}).encode())
        return self._send(404, b"no")

    def _manifest(self, svc):
        if svc == "nomain":
            return self._send(404, b'{"errors":[{"code":"MANIFEST_UNKNOWN"}]}')
        if svc == "nodig":
            # 200 with no Docker-Content-Digest at all.
            return self._send(200, b"")
        if svc == "shortdig":
            return self._send(200, b"", [("Docker-Content-Digest", "sha256:abc123")])
        return self._send(200, b"", [("Docker-Content-Digest", DIGEST)])

    do_HEAD = do_GET

srv = HTTPServer(("127.0.0.1", 0), H)
print(srv.server_port, flush=True)
srv.serve_forever()
PY

python3 "$work/stub.py" > "$work/port" &
stub_pid=$!
for _ in $(seq 1 50); do
  port=$(cat "$work/port" 2>/dev/null || true)
  [ -n "$port" ] && break
  sleep 0.1
done
[ -n "${port:-}" ] || { echo "stub registry did not start" >&2; exit 2; }
api="http://127.0.0.1:$port"

# Build a tree the script can derive `svcs` from, and run it there. `line` is
# what goes into the fake options.go: the real derivation matches
# `"ghcr.io/fil-forge/<name>:main"`, so a case that wants no fleet passes
# something that does not.
run() {
  local name=$1 line=$2 root
  root="$work/$name"
  mkdir -p "$root/.github/scripts" "$root/smelt/pkg/stack"
  cp "$under_test" "$root/.github/scripts/"
  printf '%s\n' "$line" > "$root/smelt/pkg/stack/options.go"
  set +e
  GHCR_API="$api" "$root/.github/scripts/$(basename "$under_test")" \
    > "$work/$name.out" 2> "$work/$name.err"
  echo $? > "$work/$name.rc"
  set -e
}

svc_line() { printf '\t{"ghcr.io/fil-forge/%s:main", func(c *config) *string { return &c.x }},' "$1"; }

rc_of()  { cat "$work/$1.rc"; }
out_of() { cat "$work/$1.out"; }
err_of() { cat "$work/$1.err"; }

echo "check-compat-baselines.sh"

# 1. A service with a version-shaped tag pins to it, and says nothing about
#    floating. The positive direction: without it, every negative test below
#    could pass with the script permanently broken.
run t1 "$(svc_line rel)"
if [ "$(rc_of t1)" = 0 ] && grep -qx 'base_REL=1.2.3' "$work/t1.out" \
   && ! grep -q 'sha256:' "$work/t1.out" && ! grep -q '::warning::' "$work/t1.err"; then
  ok "1 release baseline: pins the version, no warning"
else
  bad "1 release baseline (rc=$(rc_of t1)): $(out_of t1) / $(err_of t1)"
fi

# 2. No version-shaped tag falls back to :main AS A DIGEST, and warns. The
#    digest is the point: a floating TAG would leave a red unreproducible.
run t2 "$(svc_line flo)"
if [ "$(rc_of t2)" = 0 ] \
   && grep -qx "base_FLO=sha256:$(printf 'a%.0s' $(seq 64))" "$work/t2.out" \
   && grep -q '::warning::.*floating :main for flo' "$work/t2.err"; then
  ok "2 floating baseline: resolves :main to a digest and warns"
else
  bad "2 floating baseline (rc=$(rc_of t2)): $(out_of t2) / $(err_of t2)"
fi

# 3. NEWEST, and newest is numeric. `1.10.0` sorts BELOW `1.2.3` as a string,
#    so a lexicographic sort here picks the older release and nothing notices.
run t3 "$(svc_line sorting)"
if [ "$(rc_of t3)" = 0 ] && grep -qx 'base_SORTING=1.10.0' "$work/t3.out"; then
  ok "3 newest release wins, sorted numerically"
else
  bad "3 numeric sort (rc=$(rc_of t3)): $(out_of t3)"
fi

# 4. A package that does not exist is refused HERE, naming the service. It
#    used to report none and carry on, which was safe only while a missing
#    baseline meant a skip.
run t4 "$(svc_line gone)"
if [ "$(rc_of t4)" = 1 ] && grep -q '::error::gone has no package' "$work/t4.err"; then
  ok "4 no such package: exits 1 naming the service"
else
  bad "4 no such package (rc=$(rc_of t4)): $(err_of t4)"
fi

# 5. A registry that answers 500 is NOT "no releases". This is the failure the
#    `|| true` was removed to close, and it is the one worth most: it fails
#    open if it fails at all.
run t5 "$(svc_line broken)"
if [ "$(rc_of t5)" = 1 ] && grep -q "refusing to report that as" "$work/t5.err"; then
  ok "5 registry error: exits 1 rather than reporting no releases"
else
  bad "5 registry error (rc=$(rc_of t5)): $(err_of t5)"
fi

# 6. No releases AND no :main is nothing to pin to at all.
run t6 "$(svc_line nomain)"
if [ "$(rc_of t6)" = 1 ] && grep -q 'its :main' "$work/t6.err"; then
  ok "6 no release and no :main: exits 1"
else
  bad "6 no release and no :main (rc=$(rc_of t6)): $(err_of t6)"
fi

# 7. A 200 with no digest header would otherwise go out as `base_X=` and pull
#    `<pkg>@`, failing forty minutes later inside a container.
run t7 "$(svc_line nodig)"
if [ "$(rc_of t7)" = 1 ] && grep -q 'no usable Docker-Content-Digest' "$work/t7.err"; then
  ok "7 manifest without a digest header: exits 1"
else
  bad "7 missing digest header (rc=$(rc_of t7)): $(err_of t7)"
fi

# 8. Shape AND length. `sha256:abc123` passes a prefix check and is not a
#    digest; only the length check catches it.
run t8 "$(svc_line shortdig)"
if [ "$(rc_of t8)" = 1 ] && grep -q 'malformed digest' "$work/t8.err"; then
  ok "8 truncated digest: exits 1"
else
  bad "8 truncated digest (rc=$(rc_of t8)): $(err_of t8)"
fi

# 9. An options.go the derivation does not match must refuse, not resolve an
#    empty fleet and report success over it.
run t9 'var publishedImages = []struct{}{}'
if [ "$(rc_of t9)" = 2 ] && grep -q 'could not derive the service list' "$work/t9.err"; then
  ok "9 no derivable service list: exits 2"
else
  bad "9 empty service list (rc=$(rc_of t9)): $(err_of t9)"
fi

if [ "$fails" -ne 0 ]; then
  echo "check-compat-baselines.sh: $fails test(s) failed" >&2
  exit 1
fi
echo "check-compat-baselines.sh: all tests passed"
