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
#   token_for's non-403 arm, DELETED           -> FAIL (test 5)
#   token_for's non-403 arm -> `return 10`     -> FAIL (test 5)
#   the tags/list non-200 check, DELETED       -> FAIL (test 5b)
#   the `|| exit 1` on digest_for              -> FAIL (test 6)
#   the Docker-Content-Digest presence check   -> FAIL (test 7)
#   the digest length check                    -> FAIL (test 8)
#   the empty-service-list refusal             -> FAIL (test 9)
#   the pagination loop's Link following       -> FAIL (test 10)
#   numeric sorting in versions_from           -> FAIL (tests 3, 11)
#   versions_from's fullmatch (arch suffixes)  -> FAIL (tests 3, 11)
#   `baselines=` emitted as {}                 -> FAIL (tests 1-3)
#   key_for's `-` -> `_`                       -> FAIL (test 1)
#   the `[a-z0-9-]` in the service derivation  -> FAIL (test 1)
#   EITHER index media type in the Accept      -> FAIL (tests 2, 11)
#   the $GITHUB_STEP_SUMMARY block             -> FAIL (test 11)
#   the digest in the summary's floating row   -> FAIL (test 11)
#   the argument refusal                       -> FAIL (test 12)
#   digest_for's manifest non-200 arm          -> FAIL (test 13a)
#   digest_for's token-mint failure arm        -> FAIL (test 13b)
#
# Three entries name two tests because TEST 11 DRIVES THE `sorting` AND `flo`
# FIXTURES ITSELF, so anything that breaks either shows up in its summary as
# well. That is collateral, not extra coverage, and it is written down rather
# than rounded to one test each: an entry naming fewer tests than actually go
# red is how a reader concludes the wrong thing about which check is
# load-bearing.
#
# NOT IN THE LIST, because this file cannot catch it deterministically:
# putting `| head -n 1` back on newest_version_from. That is a real bug --
# `head` closing the pipe races the writer under `set -o pipefail`, and the
# script aborts -- but it is load-dependent (0/150 idle, 21/150 on a busy
# box), so a passing run here proves nothing about it. The defence is that
# the pipeline is gone, not that this file watches for it.
#
# THE SECOND AND THIRD ENTRIES ARE ONE ARM AND TWO MUTATIONS, and the
# difference is why test 5 matches the token endpoint's OWN message rather
# than the `::error::` further down. Deleting the arm outright does not stop
# the script failing: a 500 body falls through into the token parse, which
# fails, and the NEXT guard returns 1 with a different message. An earlier
# version of test 5 grepped only for that downstream `::error::`, so it passed
# on a guard other than the one it is named for -- the failure this file's own
# header warns about, happening to it.
#
# THAT LIST IS NOT EVERY GUARD IN THE SCRIPT. The transport-failure arms
# (curl itself failing in token_for, tags_for and digest_for) and the two
# body-is-not-JSON arms have no fixture, because the stub answers every
# request. An unreachable registry was checked by hand -- `GHCR_API` pointed
# at a dead port exits 1 -- but by hand is not by this file, and saying "each
# guard" would be the overclaim rule 5 is about.
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

DIGEST = "sha256:" + "a" * 64          # the multi-arch INDEX
ARCH_DIGEST = "sha256:" + "b" * 64     # one architecture's manifest

# AGENTS.md rule 2: the index digest, never a per-architecture one. The
# script asks for it by sending the two list media types in Accept, and a
# stub that ignores Accept cannot tell whether it still does -- so this one
# answers differently depending on what was asked for, and test 2 pins the
# index digest.
#
# `all`, NOT `any`. With `any`, dropping EITHER media type from the script's
# Accept left test 2 green -- including the OCI index type, which is the one
# compat-baselines.sh's own measurement rests on. A guard over part of a chain
# reads exactly like a guard over the chain (rule 5).
LIST_TYPES = ("image.index.v1+json", "manifest.list.v2+json")

TAGS = {
    # HYPHENATED ON PURPOSE. Two of the eight real services are
    # (indexing-service, piri-signing-service), and the `-` -> `_` in key_for
    # is what makes COMPAT_BASELINE_INDEXING_SERVICE line up with envSuffix's
    # own ReplaceAll -- the descendant of the COMPAT_BASELINE_SPRUE / _UPLOAD
    # mismatch this whole apparatus exists to stop. With every fixture
    # hyphen-free, both that mapping and the `[a-z0-9-]` in the service
    # derivation survive deletion with the guard fully green.
    "rel-svc": ["main", "sha-abc1234", "1.2.3"],
    # The arch-suffixed pair is ingot's real shape. WHAT IT CATCHES is a
    # loose match: `fullmatch` -> `match` takes `0.0.1-amd64` into the sort,
    # whose key does `int("0-amd64")` and raises, so the script aborts with an
    # empty stdout. Not a mis-pick -- an earlier version of this comment said
    # the pair "sorts ABOVE every real version" so a loose match would choose
    # an unrunnable reference, which does not happen and made the ordering
    # read as load-bearing when it is not. These sort BELOW everything, on
    # purpose, so nothing rests on where they fall.
    "sorting": ["1.2.3", "1.10.0", "0.9.9", "0.0.1-amd64", "0.0.1-arm64", "main"],
    "flo":     ["main", "sha-abc1234"],
    "nomain":  ["sha-abc1234"],
    "nodig":   ["main"],
    "shortdig": ["main"],
    "ratelimited": ["1.2.3"],
    "mani500": [],       # token fine, tags fine, manifest 500
    "tokflip": [],       # token fine for tags/list, refused on the manifest call
    # Two pages. The newer release is on the SECOND one, so a loop that does
    # not follow the link picks 1.0.0.
    "paged":   ["1.0.0", "main"],
}

PAGE2 = {"paged": ["2.0.0", "sha-abc1234"]}
TOKFLIP = [0]

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
            if svc == "tokflip":
                # tags_for mints one token, digest_for mints a second. The
                # first succeeds and the second is refused, which is the only
                # way to reach digest_for's own token-mint failure. A plain
                # counter is safe: this server is single-threaded and the
                # guard's cases run one at a time.
                TOKFLIP[0] += 1
                if TOKFLIP[0] > 1:
                    return self._send(403, b'{"errors":[{"code":"DENIED"}]}')
            return self._send(200, b'{"token":"stub"}')
        if "/manifests/" in self.path:
            return self._manifest(svc)
        if "/tags/list" in self.path:
            import json
            if svc == "ratelimited":
                return self._send(429, b'{"errors":[{"code":"TOOMANYREQUESTS"}]}')
            if svc in PAGE2 and "last=" not in self.path:
                body = json.dumps({"tags": TAGS[svc]}).encode()
                link = f'</v2/fil-forge/{svc}/tags/list?last={TAGS[svc][-1]}&n=1000>; rel="next"'
                return self._send(200, body, [("Link", link)])
            if svc in PAGE2:
                return self._send(200, json.dumps({"tags": PAGE2[svc]}).encode())
            return self._send(200, json.dumps({"tags": TAGS.get(svc, [])}).encode())
        return self._send(404, b"no")

    def _manifest(self, svc):
        accept = self.headers.get("Accept", "")
        wants_index = all(t in accept for t in LIST_TYPES)
        if svc == "nomain":
            return self._send(404, b'{"errors":[{"code":"MANIFEST_UNKNOWN"}]}')
        if svc == "mani500":
            return self._send(500, b"registry is having a day")
        if svc == "nodig":
            # 200 with no Docker-Content-Digest at all.
            return self._send(200, b"")
        if svc == "shortdig":
            return self._send(200, b"", [("Docker-Content-Digest", "sha256:abc123")])
        dig = DIGEST if wants_index else ARCH_DIGEST
        return self._send(200, b"", [("Docker-Content-Digest", dig)])

    do_HEAD = do_GET

# PLAIN, SINGLE-THREADED, DEFAULT BACKLOG -- which is what it was before a
# revision that flailed at a flake. That revision made this ThreadingHTTPServer
# and set `srv.request_queue_size = 128`, on a theory that the accept queue was
# the bottleneck. Both halves were wrong: the assignment comes AFTER the
# constructor, and socketserver calls `listen(self.request_queue_size)` during
# it, so the backlog stayed 5 (measured by instrumenting socket.listen);
# `daemon_threads = True` was already the class default; and the flake was
# never here at all. It was the `| head -n 1` on versions_from in the script
# under test -- see newest_version_from. Reverted rather than left in place
# with a justification that did not hold.
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
  shift 2 || true
  root="$work/$name"
  mkdir -p "$root/.github/scripts" "$root/smelt/pkg/stack"
  cp "$under_test" "$root/.github/scripts/"
  printf '%s\n' "$line" > "$root/smelt/pkg/stack/options.go"
  set +e
  GHCR_API="$api" GITHUB_STEP_SUMMARY="$work/$name.summary" \
    "$root/.github/scripts/$(basename "$under_test")" "$@" \
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
#    THE FIXTURE IS HYPHENATED, which is what makes this test cover key_for's
#    `-` -> `_` and the `[a-z0-9-]` in the service derivation. Two of the eight
#    real services are hyphenated and the mapping is what lines
#    COMPAT_BASELINE_INDEXING_SERVICE up with envSuffix.
run t1 "$(svc_line rel-svc)"
if [ "$(rc_of t1)" = 0 ] && grep -qx 'base_REL_SVC=1.2.3' "$work/t1.out" \
   && grep -qx 'baselines={"REL_SVC": "1.2.3"}' "$work/t1.out" \
   && ! grep -q 'sha256:' "$work/t1.out" && ! grep -q '::warning::' "$work/t1.err"; then
  ok "1 release baseline: hyphenated key, the version and the JSON, no warning"
else
  bad "1 release baseline (rc=$(rc_of t1)): $(out_of t1) / $(err_of t1)"
fi

# 2. No version-shaped tag falls back to :main AS A DIGEST, and warns. The
#    digest is the point: a floating TAG would leave a red unreproducible.
#
#    IT IS THE INDEX DIGEST, and the stub answers a different one when Accept
#    does not ask for the list media types -- so this also pins AGENTS.md
#    rule 2, which nothing else here could see.
run t2 "$(svc_line flo)"
if [ "$(rc_of t2)" = 0 ] \
   && grep -qx "base_FLO=sha256:$(printf 'a%.0s' $(seq 64))" "$work/t2.out" \
   && grep -qx "baselines={\"FLO\": \"sha256:$(printf 'a%.0s' $(seq 64))\"}" "$work/t2.out" \
   && grep -q '::warning::.*floating :main for flo' "$work/t2.err"; then
  ok "2 floating baseline: resolves :main to a digest and warns"
else
  bad "2 floating baseline (rc=$(rc_of t2)): $(out_of t2) / $(err_of t2)"
fi

# 3. NEWEST, and newest is numeric. `1.10.0` sorts BELOW `1.2.3` as a string,
#    so a lexicographic sort here picks the older release and nothing notices.
run t3 "$(svc_line sorting)"
if [ "$(rc_of t3)" = 0 ] && grep -qx 'base_SORTING=1.10.0' "$work/t3.out" \
   && grep -qx 'baselines={"SORTING": "1.10.0"}' "$work/t3.out"; then
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
#    `|| true` was removed to close, and the one worth most: it fails open if
#    it fails at all.
#
#    IT MATCHES THE TOKEN ENDPOINT'S OWN MESSAGE, not the `::error::` below it.
#    See the header: deleting the arm under test leaves the script failing
#    anyway, one guard further down, with a different message -- so grepping
#    the downstream text passes with the arm gone.
run t5 "$(svc_line broken)"
if [ "$(rc_of t5)" = 1 ] && grep -q 'broken: token endpoint answered HTTP 500' "$work/t5.err" \
   && grep -q "refusing to report that as" "$work/t5.err"; then
  ok "5 token endpoint error: exits 1, naming the status"
else
  bad "5 token endpoint error (rc=$(rc_of t5)): $(err_of t5)"
fi

# 5b. THE OTHER HALF OF THE SAME DEFENCE, and it had no test at all. A token
#     that succeeds and a tags/list that does not is the shape that fails
#     OPEN: without this check the service is silently demoted from a release
#     baseline to a floating one, the run stays green, and the only trace is a
#     ::warning:: naming it beside the services that float legitimately.
#     Neither the Go side's all-floating fatal nor anything else can see it,
#     because the demotion is partial.
run t5b "$(svc_line ratelimited)"
if [ "$(rc_of t5b)" = 1 ] && grep -q 'tags/list answered HTTP 429' "$work/t5b.err"; then
  ok "5b tags/list error: exits 1 rather than demoting to floating"
else
  bad "5b tags/list error (rc=$(rc_of t5b)): $(out_of t5b) / $(err_of t5b)"
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

# 10. The pagination loop, which nothing reached: every other stub answers in
#     one page. THE RISK IS A MISSED TAG, not a mis-picked one: a version-
#     shaped tag that falls on a later page is never seen at all, and the
#     service is silently demoted to a floating baseline. Nothing today
#     paginates at n=1000 -- ingot's 117 tags are the most any fleet service
#     has, and no fleet service publishes more than one version-shaped tag --
#     so this is entirely about the guarantee the script refuses to assume.
#     Page one holds the OLDER release and the Link; page two holds the newer,
#     so a loop that stops after one request picks 1.0.0.
run t10 "$(svc_line paged)"
if [ "$(rc_of t10)" = 0 ] && grep -qx 'base_PAGED=2.0.0' "$work/t10.out"; then
  ok "10 pagination: follows rel=next and sorts across pages"
else
  bad "10 pagination (rc=$(rc_of t10)): $(out_of t10) / $(err_of t10)"
fi

# 11. The job summary, which the ::warning:: points at ("the digests are in
#     the job summary; they are what makes a red reproducible"). Nothing set
#     $GITHUB_STEP_SUMMARY, so the whole block -- both arms -- was unreached
#     and could be deleted green.
#
#     IT DRIVES ITS OWN RUNS. Reading other tests' byproducts made it fail as
#     collateral whenever one of those failed for an unrelated reason: the
#     `[a-z0-9-]` mutation reddened it by making test 1 exit before writing a
#     summary at all, so the header's entry named one test and the run named
#     two.
#
#     AND IT ASSERTS THE DIGEST. Matching the row's existence and not its
#     content let the digest be replaced with a literal, green; and grepping
#     `floating` was already satisfied by the row itself, so the whole closing
#     note could be deleted, green. The digest is the entire reason that
#     ::warning:: points here.
run t11r "$(svc_line sorting)"
run t11f "$(svc_line flo)"
digest_row="| \`flo\` | \`sha256:$(printf 'a%.0s' $(seq 64))\` | **floating \`:main\`** |"
# The patterns carry backticks because the table does; nothing expands here.
# shellcheck disable=SC2016
if grep -qxF '| service | baseline | kind |' "$work/t11r.summary" 2>/dev/null \
   && grep -qxF '|---|---|---|' "$work/t11r.summary" \
   && grep -qxF '| `sorting` | `1.10.0` | release |' "$work/t11r.summary" \
   && grep -q 'Every service has a cut release' "$work/t11r.summary" \
   && grep -qxF "$digest_row" "$work/t11f.summary" 2>/dev/null \
   && grep -q "upstream's current main" "$work/t11f.summary"; then
  ok "11 job summary: the table, the digest in the row, and each closing note"
else
  bad "11 job summary: r=$(cat "$work/t11r.summary" 2>&1 | tr '\n' ' ') | f=$(cat "$work/t11f.summary" 2>&1 | tr '\n' ' ')"
fi

# 12. It takes no arguments, and used to take N. A caller still passing one
#     must be told, not silently ignored -- the same trailing-argument slip
#     finish-subtree-pull.sh had.
run t12 "$(svc_line rel-svc)" 2
if [ "$(rc_of t12)" = 2 ] && grep -q 'takes no arguments' "$work/t12.err"; then
  ok "12 an argument is refused, not ignored"
else
  bad "12 argument refusal (rc=$(rc_of t12)): $(err_of t12)"
fi

# 13. digest_for's own refusals, which had no fixture. Test 6 reached the
#     manifest arm but matched only the CALLER's message, so deleting that arm
#     left the empty-digest arm producing the same outer text.
run t13a "$(svc_line mani500)"
if [ "$(rc_of t13a)" = 1 ] && grep -q 'manifest for :main answered HTTP 500' "$work/t13a.err"; then
  ok "13a manifest non-200: exits 1, naming the status"
else
  bad "13a manifest non-200 (rc=$(rc_of t13a)): $(err_of t13a)"
fi

run t13b "$(svc_line tokflip)"
if [ "$(rc_of t13b)" = 1 ] && grep -q 'could not mint a token to resolve :main' "$work/t13b.err"; then
  ok "13b token refused at digest_for: exits 1, naming the step"
else
  bad "13b token refused at digest_for (rc=$(rc_of t13b)): $(err_of t13b)"
fi

if [ "$fails" -ne 0 ]; then
  echo "check-compat-baselines.sh: $fails test(s) failed" >&2
  exit 1
fi
echo "check-compat-baselines.sh: all tests passed"
