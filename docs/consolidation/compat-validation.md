# `compat.yml` validation without cutting tags (Experiment D)

Branch `poc/d-compat` on `fil-forge/forge`, base `f60dd59` (current `main`).
Four commits, in order: `compat: derive every service name from one table`,
`compat: prove each stack runs what the test claims`, `ci(compat): override the
window with literal image tags; make no-op runs visible`, and this document.
List them with `git log --oneline f60dd59..poc/d-compat`.

Reproduce every check in this document from the repo root (no Docker daemon
needed; Go 1.26.5, Python 3 with PyYAML):

```sh
cd smelt
GOWORK=off go vet -tags compat ./tests/compat/... ./pkg/stack/... ./pkg/workspace/...
GOWORK=off go test -tags compat -count=1 -run NONE ./tests/compat/...          # compile only
GOWORK=off go test -tags compat -count=1 -v -run 'TestServiceTable|TestCheckProvenance|TestDescribe|TestBindMountAt' ./tests/compat/
go test -tags compat -count=1 -run TestServiceTable ./tests/compat/             # with go.work active: checks the table against the workspace
GOWORK=off go test -count=1 -run 'Test[^B]' ./pkg/stack/ ./pkg/workspace/       # TestBuild*ImageTagFormat need a daemon and fail without one, at HEAD too
cd ..
python3 -c 'import yaml; yaml.safe_load(open(".github/workflows/compat.yml"))'
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 -no-color .github/workflows/compat.yml
```

Nothing in this experiment boots a container: there is no Docker daemon on the
machine that produced it. Everything below the "Verification done here" section
is prediction, labelled as such.

## Summary

- `compat.yml` can now be dispatched with literal image tags
  (`piri_versions`, `ingot_versions`, `baseline_piri`, `baseline_ingot`,
  `baseline_sprue`, `baseline_hilt`) that bypass git-tag resolution and the
  `ready` gate. The schedule path is unchanged — on a fake-tagged repo its
  `GITHUB_OUTPUT` has the same key/value set as the previous script's (the
  key order differs; `GITHUB_OUTPUT` is order-insensitive).
- The suite can no longer pass vacuously in either of the two ways it could
  before: (1) every test now proves from `docker inspect` that the pinned
  service runs its pinned image with no HEAD binary mounted over it and that
  every other in-repo service does have a HEAD binary mounted; (2) the
  workflow fails a `Version skew` job in which every test skipped.
- A nightly with nothing to test is now a visible no-op (run title, job name
  `Version skew — skipped: no releases to test against`, a `::notice`, a
  summary line) but still concludes `success`; the GitHub Actions model has no
  way to make it neutral (reasoning below).
- The `otherThan` / baseline-map naming seam is closed: one table maps compose
  name → image name → pin option, and the binary path is read from
  `pkg/workspace`'s build table, so the two cannot drift. `pkg/stack` also
  refuses an exclusion name it cannot build.
- Latent bugs found while in there are listed at the end; the two that matter
  most: the bind-mount path `compat.yml` depends on (`pkg/workspace` build +
  mount) has **zero** CI coverage anywhere — `ci.yml`'s stack job runs
  prebuilt images with `SMELT_STACK_PREBUILT=1` — and a libforge `feat!` wire
  change sits between the two available image pins and is **not** on the path
  the suite exercises.

## What changed and why

### 1. One service table (`smelt/tests/compat/compat_test.go`, `table_test.go`; `smelt/pkg/workspace/workspace.go`; `smelt/pkg/stack/stack.go`, `stack_test.go`)

At `f60dd59`, `otherThan()` listed smelt's compose names
`{"piri", "ingot", "upload", "hilt"}` while `TestRollingUpgrade` iterated the
registry's image names `{"piri", "ingot", "sprue", "hilt"}` for the baseline
map. They disagree on sprue (compose service `upload`, image `sprue`, binary
`/usr/bin/sprue`). Nothing tied the two lists together: passing `"sprue"` to
`stack.WithWorkspaceBinariesExcept` would have excluded nothing, the
working-tree sprue would have been mounted over the pinned image, and the run
would have quietly become HEAD-vs-HEAD.

Now `services` is the only place the four in-repo services are spelled:

| compose name | image name (`ghcr.io/fil-forge/forge/<image>`, `COMPAT_<IMAGE>_*`) | pin option | operator-run |
|---|---|---|---|
| `piri` | `piri` | `stack.WithPiriImage` | yes |
| `ingot` | `ingot` | `stack.WithIngotImage` | yes |
| `upload` | `sprue` | `stack.WithUploadImage` | no |
| `hilt` | `hilt` | `stack.WithHiltImage` | no |

`otherThan`, the pinned-peer loop, `baselineSet` and every `COMPAT_*` key are
derived from it. The binary path is deliberately **not** a fifth column: it is
read through the new `workspace.BinPath(composeName)` from the same
`workspace.Services` table that `RenderOverride` mounts from, so the check
and the mount cannot disagree. `TestServiceTable` (no daemon needed) verifies
every row is buildable, the exclusion list is exactly "the other rows", and —
when a `go.work` is active, as in the compat job — that the set of services
the workspace would build from HEAD equals the table.

Defence in depth: `stack.NewStack` now returns an error for a
`WithWorkspaceBinariesExcept` name that `pkg/workspace` cannot build, before
`workspace.Detect` runs (so it fires with no daemon and no `go.work`).
`TestWorkspaceExcludeUnknownService` covers it with the literal `"sprue"` case.
Blast radius: `pkg/stack` is imported by services' own test suites; a caller
already passing a wrong name was already getting a silent no-op, and now gets
an error naming the known services.

### 2. Provenance assertion (`smelt/pkg/stack/inspect.go`; `compat_test.go`, `provenance_test.go`)

`stack.Inspect(ctx, service)` returns a stack-owned `ContainerInfo{Image,
Mounts}` built from `docker inspect` (`.Config.Image`, `.Mounts`) via the
compose stack's `ServiceContainer(...).Inspect(...)`; `stack.PiriServiceNames`
enumerates the `piri-N` fan-out. No docker/moby API types leak to callers
(the first draft imported `github.com/docker/docker/api/types/container`;
`testcontainers-go v0.42.0` returns `github.com/moby/moby/api` types, so it
did not compile).

`checkProvenance(ctx, stack, want)` walks every row of the table and, for each
container:

- pinned service: `Image` must equal the pinned ref exactly, and there must be
  **no** bind mount at its binary path;
- HEAD service: there **must** be a bind mount at its binary path (only type
  `bind` counts — a volume there is not a host-built binary).

All mismatches are collected (`errors.Join`) so one failure shows the whole
picture, and `assertProvenance` fails the test with them. It runs before
`assertUploadRetrieve`, so a mislabelled stack fails before it can pass
anything. After upload/retrieve, each test logs one line
`compat: exercised shape=<pinned-peer|rolling-upgrade> pinned=<compose>@<ref>,… head=<compose>,…`
which the workflow counts.

`TestCheckProvenance` drives the guard from a fake inspector through 12 cases
(correct pinned peer; HEAD binary mounted over the pin; wrong image; wrong
image plus mount; a HEAD service without its mount; a volume instead of a bind
mount; a two-node rolling upgrade, correct and with one node missing the
binary; the old `otherThan` seam scenario where sprue was not held back; every
container wrong; an un-inspectable container; a stack with no piri nodes).
`TestProvenanceGuardFires` is the negative end-to-end check the plan asks
for: gated on `COMPAT_EXPECT_PIN_MISMATCH=1`, it pins piri but calls
`stack.WithWorkspaceBinaries()` with no exclusion, and asserts the guard
rejects the resulting stack *on the bind-mount branch*: the error must name
`bind-mounted over /usr/bin/piri` and must not also report `created from`
(an image-string mismatch is a separate defect that run A would surface on
its own). It is skipped by default (it boots a full stack
for no compatibility signal) and is reachable from the workflow through the
`expect_pin_mismatch` input.

`COMPAT_GUPPY_IMAGE` (workflow input `guppy_image`) optionally pins the guppy
client image; smelt's compose default `ghcr.io/fil-forge/guppy:main-dev`
floats, which otherwise makes a guppy drift indistinguishable from a service
skew.

Residual gap, not closed: a bind mount at `/usr/bin/<svc>` proves the file is
there, not that PID 1 executes it. Every published image today does execute
that path (the shared `docker/Dockerfile` symlinks its entrypoint to
`/usr/bin/${SERVICE}`; the `sha-96a672e` per-service Dockerfiles used
`ENTRYPOINT ["/usr/bin/<svc>", …]`; piri's `entrypoint.sh` ends in
`exec /usr/bin/piri serve full …`), so the check is sound for the images that
exist. A future image that moved the binary would defeat it; the cheap next
step is an `Exec` of `readlink -f /proc/1/exe` compared with the binary path.
Not added, to keep the first real run free of a second untested mechanism.

### 3. Workflow (`.github/workflows/compat.yml`)

- `workflow_dispatch` inputs: the six override tags, plus `window` (unchanged),
  `guppy_image` and `expect_pin_mismatch` (boolean). All string inputs default
  to `''`; on a `schedule` event the `inputs` context is empty, so the script
  sees `''` for all of them and takes the tags path.
- Resolve step: inputs go through `env` (`IN_*`) so a value can only be data;
  whitespace is stripped (`"a, b"` → `a,b`); if any of the six is non-empty,
  `mode=override`, `ready=true`, values used verbatim, empty inputs left empty.
  Otherwise the previous tag-resolution code runs unchanged (`mode=tags`).
  `set -euo pipefail` kept. Two new outputs: `mode`, `guppy_image`.
- Report step: writes the resolved window as a table to the job summary
  (`GITHUB_STEP_SUMMARY`); when `ready != true` it emits
  `::notice title=Compatibility skipped: no releases::…` and appends
  `**skipped: no releases.**` to the summary.
- `run-name`: `Compatibility · nightly (tags mode)` on schedule; on dispatch,
  `Compatibility · override piri=[…] ingot=[…] baseline piri=… ingot=… sprue=… hilt=…`
  or `Compatibility · dispatch (tags mode, window=N)`. Written as a folded
  block scalar because the expression contains `: `, which a YAML plain scalar
  rejects — the draft that inlined it did not parse.
- `compat` job: name is an expression on `needs.window.outputs.ready`
  (`Version skew` vs `Version skew — skipped: no releases to test against`);
  `if:` unchanged. New steps: pre-pull every pinned ref (a missing tag fails
  here, with the ref in the message, instead of as a stack-startup timeout);
  run the suite under `shell: bash` (pipefail) through `tee` to
  `/tmp/suite-compat.log` and the repo's `group-go-tests.awk`; **refuse a
  vacuous pass** (`grep -c 'compat: exercised '` = 0 → `::error`, exit 1); job
  summary with pass/fail/skip counts, the exercised configurations and the
  skipped tests. Log dump and artifact upload on failure unchanged.
- Header comments rewritten: both modes, the 2026-09-03 status (nightly since
  2026-08-01, always green, never executed a test), and why the no-op run
  stays green.

Why the no-op cannot be neutral: a job's conclusion is one of `success`,
`failure`, `cancelled`, `skipped`. `skipped` comes only from a job-level `if`,
evaluated before the job starts from contexts such as `needs` — a job cannot
inspect the repository and then skip itself. The neutral exit code (78) was
removed from Actions in 2019. A run shows as skipped only when every job is
skipped, but deciding whether there are tags needs a job that runs, so at
least one job always succeeds. `continue-on-error` yields `success` with an
annotation. The Checks API can create a `neutral` check run, but that needs
`checks: write`, creates a separate check, and leaves the workflow run itself
green — more misleading, not less. The only red option is failing the window
job when no tags exist; the original author rejected a permanently red
nightly until the first release and this branch keeps that decision. Hence:
visible, not grey.

## Verification done here (measured in this session)

| check | command | result |
|---|---|---|
| gofmt | `gofmt -l smelt/pkg/stack smelt/pkg/workspace smelt/tests/compat` | only `pkg/stack/proofs.go`, which is not gofmt-clean at `f60dd59` either (untouched) |
| vet | `GOWORK=off go vet -tags compat ./tests/compat/... ./pkg/stack/... ./pkg/workspace/...` | clean |
| compile | `GOWORK=off go test -tags compat -count=1 -run NONE ./tests/compat/...` | `ok … [no tests to run]` |
| daemon-free suite tests | `TestServiceTable`, `TestCheckProvenance` (12 subtests), `TestDescribe`, `TestBindMountAt` | all PASS, with `GOWORK=off` and with the repo `go.work` active (the latter also checks table == workspace selection: `{hilt, ingot, piri, upload}`) |
| stack/workspace units | `GOWORK=off go test -count=1 -run 'Test[^B]' ./pkg/stack/ ./pkg/workspace/` | PASS (incl. `TestWorkspaceExcludeUnknownService`). The excluded `TestBuild{,Piri,Guppy}ImageTagFormat` call `docker build` and fail without a daemon — verified identical at `f60dd59` in a throwaway worktree |
| YAML | PyYAML 6.0.1 `safe_load` | parses; 9 dispatch inputs; `run-name` is one line (584 chars); job name and `if` as intended |
| actionlint | v1.7.12 (built from source), `-no-color` | no findings on `compat.yml`; also none on `ci.yml`, `publish-ghcr.yml`, `release.yml`, `docs-site.yml`. shellcheck integration inactive (no `shellcheck` binary here) |
| Resolve script, override | inputs `" sha-96a672e, sha-f60dd59 "` etc. via env, bash | `mode=override ready=true piri=sha-96a672e,sha-f60dd59 ingot=` (empty stays empty) |
| Resolve script, tagless | temp repo, no tags, all `IN_*` empty | `mode=tags ready=false`, all values empty |
| Resolve script, equivalence | temp repo with `piri/v0.1.0 v0.2.0 v0.10.0`, `ingot/v1.0.0 v1.1.0`, `sprue/v0.3.0`, `hilt/v0.0.1`; new vs `f60dd59` script | same `GITHUB_OUTPUT` key/value set for N=2 and N=1 (ignoring the two new keys `mode`, `guppy_image`; the key order differs, which `GITHUB_OUTPUT` does not care about); `piri=v0.10.0,v0.2.0` — `--sort=-v:refname` orders semver correctly |
| Resolve script, many tags | temp repos with 300 / 600 / 1000 `piri/v*` tags, `set -euo pipefail` | `f60dd59`'s `head -n` pipeline exits 141 (SIGPIPE) from 600 tags up; the `awk` pipeline exits 0 with the same first N — latent bug 17 |
| pre-pull script | docker stubbed | pulls the de-duplicated list of 6 refs; "nothing pinned" path exits 0 |
| vacuous-pass guard | synthetic logs | exit 0 with one `compat: exercised` line; exit 1 with `::error` when all tests skipped |
| summary / report scripts | synthetic inputs | render the counts table, exercised list, skipped list; `::notice` emitted when `ready=false` |
| images exist | anonymous GHCR token + `/v2/<repo>/tags/list` (read-only GET) | `ghcr.io/fil-forge/forge/{piri,ingot,sprue,hilt}`: tags `main, main-dev, sha-96a672e, sha-f60dd59` (F10 reproduced; the packages are publicly readable). `ghcr.io/fil-forge/guppy`: `main, main-dev` and 13 `sha-*` / `sha-*-dev` pairs |
| local tags | `git tag --list \| wc -l` in this checkout | 0 (F9's premise) |

## The dispatch the coordinator should run

Ref: the POC branch that contains these commits (the workflow file is read
from the dispatched ref, so the new inputs exist there). Workflow file:
`.github/workflows/compat.yml`. Do **not** dispatch `publish-ghcr.yml`.

Run A — the primary validation (both shapes, 4 stacks):

```sh
gh workflow run compat.yml --repo fil-forge/forge --ref <poc-branch> \
  -f piri_versions=sha-96a672e \
  -f ingot_versions=sha-96a672e \
  -f baseline_piri=sha-f60dd59 \
  -f baseline_ingot=sha-f60dd59 \
  -f baseline_sprue=sha-f60dd59 \
  -f baseline_hilt=sha-f60dd59
```

Expected run title: `Compatibility · override piri=[sha-96a672e] ingot=[sha-96a672e] baseline piri=sha-f60dd59 ingot=sha-f60dd59 sprue=sha-f60dd59 hilt=sha-f60dd59`.

Run B — proof the suite can fail (adds `TestProvenanceGuardFires`, one more
stack; can be combined with A by adding the flag to it):

```sh
gh workflow run compat.yml --repo fil-forge/forge --ref <poc-branch> \
  -f piri_versions=sha-96a672e -f expect_pin_mismatch=true
```

Run C (optional, only if A is green) — old baseline for the rolling upgrade,
i.e. HEAD piri/ingot against `sha-96a672e` sprue and hilt. This is the one
that meets the packaging drift described under "Anticipated failure modes":

```sh
gh workflow run compat.yml --repo fil-forge/forge --ref <poc-branch> \
  -f baseline_piri=sha-96a672e -f baseline_ingot=sha-96a672e \
  -f baseline_sprue=sha-96a672e -f baseline_hilt=sha-96a672e
```

The workflow's `concurrency` group is the workflow name: a dispatch during the
07:00 UTC nightly queues behind it.

## What to expect (the D agent's predictions, written before any run; see Results)

Mechanics common to every test, in the order they can fail:

1. `window` job: seconds. Summary table shows the six values above,
   `mode=override ready=true`.
2. `Pre-pull pinned images`: six pulls (piri and ingot at both tags; sprue and
   hilt at `sha-f60dd59`). All exist and are public (measured), so this should
   pass in a minute or two.
3. `Run compatibility suite`, per stack: `pkg/workspace` builds the HEAD
   binaries on the runner (`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`, piri with
   `-tags skiff`) into the test's temp dir, then compose brings up ~20
   containers (postgres ×3, vault, dynamodb-local, minio, anvil, indexer,
   delegator, plc, email, signer, the four services, guppy) and waits for
   health with a 5-minute budget (`stack.WithTimeout` default). Then
   `assertProvenance`, then login → space → 10 MB upload with one replica →
   retrieve through the guppy container. Four stacks run sequentially
   (`-parallel 1`).
4. Timing (estimate, not measured): first piri build is the long pole — the
   planning brief's cold build of all six forge modules on this machine was
   245 s (F6, not re-measured here); on a 4-vCPU runner expect several
   minutes, then seconds for rebuilds thanks to the Go build cache. Each stack
   2–5 minutes to healthy plus 1–3 minutes of upload/retrieve. Total roughly
   20–40 minutes against the 55-minute `go test -timeout` and 60-minute job
   timeout. If it times out, the first thing to shorten is the number of
   configurations per run, not the timeouts.

Per test, for run A:

- `TestPinnedPeer/piri/sha-96a672e` — piri at `sha-96a672e`, HEAD hilt, ingot,
  sprue mounted over their `:main` images (`:main` is `f60dd59` content; forge
  `main` has not moved since 2026-08-01, F8). Between the two pins piri
  changed 10 files (+44/−72, build and module wiring); libforge moved from
  `5e299c46f62f` to `928cf2a21b7e`, two commits: `feat!: add cause to blob
  release arguments (#50)` and `feat: adopt Hilt's S3 client … (#52)`. Neither
  touches `blob/allocate`, `blob/accept`, `ucan/conclude` or `index/add`, and
  `blob/release` is only sent on `blob/remove`, which the smoke path never
  issues. **Expected: pass** (wire-compatible on the exercised path). The old
  piri image is `alpine` with `USER nobody`; the generated compose forces
  `user: "0:0"` and its healthcheck uses busybox `wget`, and smelt's
  `register-did.sh` was made POSIX-only in `f60dd59`, so the old image should
  boot under HEAD's compose.
- `TestPinnedPeer/ingot/sha-96a672e` — **expected: pass, and it proves very
  little.** `assertUploadRetrieve` only waits for ingot's `/health` (present
  at both revisions: `s3api.WithHealth("/health")`) and then drives guppy →
  sprue → piri; ingot's own wire contract with hilt and sprue (its did:web
  client, `/s3/request/authorize`, the hilt S3 client that PR #52 moved into
  libforge) is not exercised at all. Ingot's boot-time coupling is only
  hilt's `post_start.sh` registering ingot as a provider via hilt's CLI, in
  the hilt container. If this subtest fails, look at ingot's startup logs
  first (config parse — the ingot compose and config are unchanged between
  the pins).
- `TestRollingUpgrade/upgrade_piri` and `upgrade_ingot` with the
  `sha-f60dd59` baseline — the HEAD binary and the baseline image are the
  same source commit (the POC branch does not touch service code), so these
  are HEAD-vs-itself. **Expected: pass.** Their value in run A is
  mechanical: they exercise pinning all four services at once, mounting one
  HEAD binary over an image of the same commit, and the provenance check on a
  fully pinned fleet. They say nothing about compatibility until a baseline
  other than HEAD's commit is used (run C, or the first release).
- `TestProvenanceGuardFires` — skipped in run A (env unset). In run B:
  **expected: pass**, by observing `checkProvenance` reject
  `piri-0: pinned to …sha-96a672e but a binary is bind-mounted over
  /usr/bin/piri (from /tmp/…/piri)`. If it instead reports "guard did NOT
  fire", the bind-mount path or `Config.Image` semantics differ from what the
  fake tests assume — the most important thing the run can teach us.

Anticipated failure modes, most likely first:

1. **The bind-mount path had never run in CI** (it has now: runs 35 and 36,
   see Results — it worked first time). `ci.yml`'s stack job sets
   `SMELT_STACK_PREBUILT=1` and `GOWORK=off` and runs the images as built —
   "there is no binary-mounting layer in CI" (its own comment). The e2e suite
   only calls `WithWorkspaceBinaries` when `SMELT_WORKSPACE` is set (never in
   CI). `compat.yml` is the only CI consumer of `WithWorkspaceBinaries*`, and
   it never executed. So `workspace.Detect` (`go env GOWORK` from `smelt/`
   must find the repo-root `go.work` — it should, Go walks up),
   `BuildBinary` on a runner (module downloads for four modules; the
   `setup-go` cache is keyed on the five `go.sum` files), and `RenderOverride`
   are all first-run code in this environment. A failure here reads
   `smeltery: failed to create stack: workspace binaries: …` before any
   container starts.
2. **Cold image pulls inside the 5-minute startup budget.** Only the pinned
   refs are pre-pulled; the `:main` images for the HEAD services in
   `TestPinnedPeer` and every third-party image are pulled by compose during
   `Up`. `ci.yml`'s stack job has the same exposure and was green at
   `f60dd59` (F11), so this is probably fine; if `start stack: … wait for
   …` times out on the first stack only, this is why. Mitigation: pre-pull
   `:main` for the four services too, or raise `stack.WithTimeout`.
3. **guppy `main-dev` drift.** The client image floats; `smelt/pkg/clients/guppy`
   drives its CLI by flags. A failure in `login:`, `generate space:`,
   `upload:` or `retrieve:` that reproduces across all four stacks is guppy,
   not skew. Re-run with `-f guppy_image=ghcr.io/fil-forge/guppy:<sha-…-dev>`
   (13 such tags exist; the `-dev` variant is what the compose default uses,
   and the compose overrides the entrypoint with `sleep infinity` so the
   image only needs a shell and the `guppy` binary).
4. **`Config.Image` not equal to the pinned ref.** The provenance check
   compares `docker inspect .Config.Image` with the exact string passed to
   `WithPiriImage` etc. Compose passes the interpolated `image:` string
   through unchanged for fully-qualified `ghcr.io/…` references, but this is
   the first time the assumption is tested against real compose output. The
   failure would read `pinned to X but the container was created from "Y"`
   with Y visibly the same image spelled differently — then the check should
   normalise, not the pin.
5. **Provenance check for the HEAD services finds no mount** — would mean
   `RenderOverride` mounted at a path other than `workspace.Services[...].binPath`
   or the override file was not applied. Both are read from the same table,
   so this would point at compose file precedence, not at the table.
6. **Packaging drift between the pins (run C only).** The `sha-96a672e` sprue
   and hilt images were built from per-service Dockerfiles with
   `ENTRYPOINT ["/usr/bin/<svc>", "serve"]`; HEAD's compose passes
   `command: ["serve", …]` for the shared Dockerfile's bare-binary entrypoint,
   so an old image runs `hilt serve serve` / `sprue serve serve --config …`.
   For sprue this is not new: `smelt/systems/upload/compose.yml` already had
   `command: ["serve", "--config", …]` at `96a672e`, so the `96a672e` sprue
   image ran `serve serve` in its own era; only hilt's `command: ["serve"]`
   was added in `f60dd59`.
   Both `serve` commands are cobra commands with no `Args:` validator (checked
   at `96a672e`), so the stray positional argument should be ignored — but if
   run C fails at boot for hilt or sprue with a CLI error, this is the cause,
   and it is a real finding: smelt HEAD's compose files are not compatible
   with the images the network was running five weeks ago.
7. **Runner resources.** Four sequential ~20-container stacks plus Go builds
   on a 4-vCPU runner; each stack is torn down (volumes removed) before the
   next. The e2e suite runs two such stacks concurrently in CI, so one at a
   time should fit.

## Reading the results — checklist

1. Run title says `override` and lists the six values you passed.
2. `Resolve supported window` job summary: `mode override`, `ready true`, the
   table matches the inputs, guppy row `(empty; …)` unless you pinned it.
3. Jobs list: `Version skew` is present and ran (not the `— skipped` name).
4. `Pre-pull pinned images` step: six `docker pull` groups; any failure here is
   a tag/registry problem, not a compatibility result.
5. `Run compatibility suite` log, per stack, in order: `smeltery: built <svc>
   from local source` for the three (pinned-peer) or one (rolling-upgrade)
   HEAD services and `smeltery: NOT building <svc> from local source` for the
   pinned ones; `compat: provenance verified: pinned=… head=…`; then
   `compat: exercised shape=…`. Provenance lines must show
   `pinned=piri@ghcr.io/fil-forge/forge/piri:sha-96a672e head=hilt,ingot,upload`
   and the mirror image for ingot.
6. `Refuse a vacuous pass` prints `exercised configurations: 4` for run A
   (2 pinned-peer + 2 rolling-upgrade). Anything less means a skip you did
   not intend; the summary's "skipped" list names it.
7. `Version skew` job summary: passed/failed/skipped counts and the four
   exercised lines. Run A should show 1 skipped (`TestProvenanceGuardFires`).
8. On failure, classify by the first failing line: `workspace binaries:` →
   build/mount mechanics (failure mode 1); `start stack:` → boot, read the
   `=== container logs: <name> ===` blocks the stack dumps and the
   `compat-container-logs` artifact; `provenance mismatch` → the mounting or
   pinning did not do what the test believed (failure modes 4–5); `login:` /
   `upload:` / `retrieve:` → guppy or wire (failure mode 3, then real skew);
   `::error title=Compatibility suite exercised nothing` → env plumbing.
9. For run B: `TestProvenanceGuardFires` must PASS with a log line
   `provenance guard fired as expected:` followed by the `piri-0: pinned to …
   but a binary is bind-mounted over /usr/bin/piri` message. A FAIL here says
   the guard is blind and everything in run A that depended on it is suspect.
10. Record wall-clock times of the window job, the pre-pull step, the first
    stack (includes the cold piri build) and a later stack — they decide
    whether four configurations per nightly is affordable.

## Latent bugs and things that look wrong

Found while reading; none fixed beyond what the commits above describe.

1. **F9 — green nightly, zero tests.** Since 2026-08-01 every scheduled run
   concluded `success` with `Version skew` skipped because no `<svc>/v*` tag
   exists (34 runs per the POC brief's F9, a coordinator measurement not
   repeated here; this checkout has 0 tags). The plan said the workflow "has
   never run"; it has run every night and never executed a test. Addressed
   here as far as the Actions model allows (visible no-op, vacuous-pass
   guard) — not made red.
2. **`otherThan` / baseline naming seam** (`compose` vs `image` names for
   sprue). Closed by the table; a stack-level guard now also refuses unknown
   exclusions.
3. **Vacuous pass through a missing exclusion.** `WithWorkspaceBinariesExcept`
   degrading to HEAD-vs-HEAD was flagged in a comment and enforced nowhere.
   Closed by `assertProvenance`.
4. **`pkg/workspace` build + bind-mount has no CI coverage.** `ci.yml` runs
   prebuilt images (`SMELT_STACK_PREBUILT=1`), e2e mounts only with
   `SMELT_WORKSPACE` set locally, and `compat.yml` never ran. The path
   `compat.yml` depends on is exercised only on developers' machines.
5. **`go.work` must not list a shared module for compat to work — triggered
   by Experiment C, fixed in `8bcbddc`.** At `f60dd59`, `workspace.Detect`
   selected every service when `libforge` was in the use-list — including
   `indexer`, `delegator`, `guppy`, `signing-service`, whose module dirs do
   not exist in forge — so `BuildBinary` would fail. Experiment C (`24ae12e`)
   applied the same rule to the in-repo `commands` and `internal` modules,
   which the monorepo's `go.work` always lists; when the two branches were
   combined, `TestServiceTable` failed exactly as predicted here (measured:
   `the workspace would build [delegator guppy hilt indexer ingot piri
   signing-service upload] from HEAD; the compat table covers [hilt ingot piri
   upload]`). `8bcbddc` makes `Detect` select only the services whose module
   is in the use-list and pins that with `TestDetectSelectsOnlyWorkspaceModules`;
   after it, `TestServiceTable` passes with the branch's `go.work` active.
6. **Wire change between the pins not on the tested path.** libforge
   `b13386b feat!: add cause to blob release arguments (#50)` changed
   `/blob/release`, which sprue sends to piri on `blob/remove`. The smoke path
   (`assertUploadRetrieve`) never removes a blob, so `sha-96a672e` piri vs HEAD
   sprue will pass regardless of whether the old piri accepts the new `cause`
   argument. This is the plan's question 7 (suite depth) made concrete: the
   compat suite inherits e2e's smoke-only coverage and would not have caught
   the one `feat!` in the window.
7. **`TestPinnedPeer/ingot` tests almost nothing about ingot.** Only `/health`
   is checked; ingot's S3 surface and its hilt/sprue clients are not driven.
   The s3 suite (`smelt/tests/s3`) has the scenarios; compat does not reuse
   them.
8. **`TestRollingUpgrade` does not upgrade in place.** Its doc says "boot the
   whole stack at a released version, then replace ONE service with HEAD in
   place"; the code boots the mixed fleet directly, with no state carried from
   the old binary to the new one. It tests old-fleet/new-service
   interoperability, not an upgrade.
9. **Compose command/entrypoint contract drift.** HEAD's `hilt/compose.yml`
   gained `command: ["serve"]` in `f60dd59` for the shared Dockerfile's
   bare-binary entrypoint; `sha-96a672e` hilt and sprue images bake `serve`
   into `ENTRYPOINT`. The old hilt image under new compose runs `serve serve`;
   the old sprue image already did so under its own compose at `96a672e`
   (`upload/compose.yml` had `command: ["serve", "--config", …]` there).
   Tolerated only because neither `serve` command validates positional
   arguments.
10. **`workspace.BuildBinary` hardcodes `GOARCH=amd64`** while the s3 suite's
    `localIngotBinary` uses `runtime.GOARCH`. Fine on GitHub's amd64 runners;
    on an arm64 host the mounted binary would not execute in the (multi-arch)
    image.
11. **guppy client floats** (`ghcr.io/fil-forge/guppy:main-dev`, F10) in a
    suite whose purpose is controlling versions. `guppy_image` /
    `COMPAT_GUPPY_IMAGE` now allow pinning; the default still floats.
12. **Only pinned refs are pre-pulled**; the HEAD services' `:main` images and
    all third-party images are pulled inside the 5-minute stack budget
    (failure mode 2).
13. **`smelt/pkg/stack/proofs.go` is not gofmt-clean at `f60dd59`.**
14. **`smelt/pkg/stack` unit tests need a daemon**: `TestBuild{,Piri,Guppy}ImageTagFormat`
    run `docker build`, so `go test ./pkg/stack/` fails on any daemon-less
    host (passes on `ubuntu-latest`, which has Docker).
15. **F13 restated for this branch**: `ci.yml` path filters select `smelt`
    for these commits (`smelt/**` changed), so a push of this branch runs
    `unit smelt` and the `stack` job (e2e + s3 on prebuilt images); a change
    touching only `.github/workflows/compat.yml` or `docs/**` would run no CI
    at all.
16. **Uncommitted draft found in this worktree** at start (timestamps ~19:37–
    19:41 UTC 2026-09-03; a previous, interrupted attempt at this task). Its
    `inspect.go` imported the wrong inspect type and did not compile, and its
    `compat.yml` `run-name` was not valid YAML; the design was largely kept,
    both defects fixed, and the draft is preserved under the session scratch
    directory (`exp-d-compat/draft-backup/`) for comparison. Nothing from it
    was committed as-is.
17. **`Resolve supported window` would die of SIGPIPE at ~300 tags per
    service — pre-existing, fixed here.** Both `f60dd59`'s script and the
    first version of this one ran `git tag --list … | head -n "$N" | …` inside
    `$(…)` under `set -euo pipefail`. Once a service's tag list exceeds one
    stdio buffer (~4 KB, about 300 `piri/vX.Y.Z` tags) `head` exits early,
    `git` takes SIGPIPE, `pipefail` fails the substitution and the step exits
    141 — the nightly would go red for a plumbing reason. Measured on temp
    repos: 300 tags exit 0, 600 and 1000 tags exit 141, for both scripts. The
    `awk 'NR <= n'` form reads everything and exits 0 at every size.
18. **Log timestamps in the suite step are flush times, not event times.**
    `.github/scripts/group-go-tests.awk` writes through a 4 KiB stdout buffer,
    so the GitHub timestamp of every line falls on a buffer boundary (every
    jump in run 35 sits at a cumulative offset of 4013–4067 bytes mod 4096);
    a stack's four `smeltery: built …` lines carry one timestamp. Per-build
    durations cannot be read from the log. `fflush()` per line in the awk
    script (or `stdbuf -oL`) fixes it; also affects `ci.yml`, which uses the
    same script.
19. **ucantone's container decoder hides why a receipt was not a receipt.**
    `container.decodeTokens` discards the `receipt.Decode` error, so a
    post-#49 client facing a pre-#49 executor reports `missing receipt for
    task` and the server logs a 200 — nothing in either log names the
    audience rule. Cross-repo (ucantone); worth an issue there regardless of
    layout.
20. **`Refuse a vacuous pass` does not run when the suite fails.** Harmless —
    a red suite is not a vacuous pass — but the exercised-configuration count
    only gates green runs. `if: always()` plus a check on the suite's outcome
    would make the summary complete on red runs too.
21. **Four components of every compat stack float.** `guppy:main-dev`,
    `indexing-service:main`, `delegator:main` and
    `piri-signing-service:main` are the compose defaults
    (`smelt/systems/*/compose.yml`); only guppy has a compat input. Runs 35
    and 36 differ by the guppy pin alone and disagree on two of four stacks.
    The floating indexer (post-#49 today) is what failed `upgrade_ingot` in
    run 36. Pin all four by `sha-*` tag or digest, or the suite's answer
    changes with other repositories' pushes.

## First real runs — what was dispatched, and pre-registered predictions

Written on 2026-09-04 at 04:00 UTC, after dispatch and **before any log of
either run had been read** (the commit that adds this section predates the
one that records results). Head at dispatch: `cbca261`; suite and workflow as
of `d57234a`.

| run | dispatched (UTC) | inputs | purpose |
|---|---|---|---|
| [35 · 33834794514](https://github.com/fil-forge/forge/actions/runs/33834794514) | 03:52:54 | `piri_versions=sha-96a672e ingot_versions=sha-96a672e baseline_piri/ingot/sprue/hilt=sha-f60dd59 expect_pin_mismatch=true`; guppy at the compose default `ghcr.io/fil-forge/guppy:main-dev` (floating; today guppy `d74fd06`, ucantone `3a20cd5`) | runs A and B above in one dispatch — the guard test boots its own stack, so it cannot confound the others, and `compat.yml` serialises runs (`cancel-in-progress: false`), so two dispatches would only have added a cycle |
| [36 · 33834923448](https://github.com/fil-forge/forge/actions/runs/33834923448) | 03:55:11 | same pins; `guppy_image=ghcr.io/fil-forge/guppy:sha-c94d43b-dev` (guppy 2026-07-31, ucantone `7985ec0` of 06-19, libforge `2b55dbc` — the guppy generation forge `main`'s last green stack run used, modulo dependabot bumps); no guard | remove the guppy-side skew (S9) so that what fails is HEAD-vs-field-image, not client drift |

Observed before writing this (job step timestamps, not logs): run 35's
`Resolve supported window` resolved `mode=override ready=true` in 6 s; the
`Version skew` job pre-pulled all six pinned refs in 11 s and started the
suite at 03:53:31. So the override plumbing and GHCR access work; everything
below is prediction.

### Why the predictions above no longer apply as written

They assumed HEAD's services carry `f60dd59`'s dependencies. On this branch
HEAD carries Experiment A's bump to ucantone `8d7eb73` (post-#49), and
`bfc05d9` (#49) made `receipt.Decode` reject any receipt that carries `aud`,
while every executor built against ucantone `ccb7705` — which is what *all*
`sha-96a672e` and `sha-f60dd59` images run (checked in each service's
`go.mod` at both commits) — sets `aud` on every receipt. The pre-#49 decoder
has no audience check at all. The break is therefore one-directional: a
post-#49 component cannot read a pre-#49 executor's receipts; a pre-#49
component reads post-#49 receipts fine. The upload path is
guppy → sprue → piri (`/blob/allocate`, `/blob/accept` receipts); hilt and
ingot are not on it, so their pinned or upgraded state should not matter to
`assertUploadRetrieve`.

### Predictions

| test | run 35 (guppy post-#49) | run 36 (guppy pre-#49) | reasoning |
|---|---|---|---|
| `TestPinnedPeer/piri/sha-96a672e` | **FAIL** | **FAIL** | HEAD sprue must decode old piri's `aud`-bearing receipts and rejects them — a real wire skew between HEAD and the image the network ran five weeks ago |
| `TestPinnedPeer/ingot/sha-96a672e` | PASS | PASS | ingot is off the path; HEAD sprue and piri agree; either guppy decodes HEAD's `aud`-less receipts |
| `TestRollingUpgrade/upgrade_piri` | **FAIL** | PASS | run 35 fails on S9 (new guppy vs old sprue's receipts); in run 36 HEAD piri's `aud`-less receipts decode in old sprue |
| `TestRollingUpgrade/upgrade_sprue` | **FAIL** | **FAIL** | HEAD sprue vs old piri — the same skew as the first row |
| `TestRollingUpgrade/upgrade_ingot` | **FAIL** | PASS | run 35: S9; run 36: every component on the path is pre-#49 |
| `TestRollingUpgrade/upgrade_hilt` | **FAIL** | PASS | as for ingot |
| `TestProvenanceGuardFires` | PASS (fires on the bind-mount branch) | skipped | pinned piri with HEAD mounted over it |
| provenance guard in the other tests | passes in every stack | passes | the mechanics were the D agent's prediction and stand |
| `Refuse a vacuous pass` | passes (exercised lines present) | passes | |
| job conclusion | **failure** | **failure** | at least one test fails in each |

If instead every stack fails before `assertUploadRetrieve` runs (no
`compat: exercised` line, provenance errors on every service, or a timeout
waiting for health), the bind-mount path (latent bug 11) or the compose
contract with old images (latent bug 9) is the cause, not the wire; the
results section below says which happened.

## Results

### Run 35 — HEAD vs the old images, guppy `main-dev`, guard enabled

[33834794514](https://github.com/fil-forge/forge/actions/runs/33834794514):
dispatched 03:52:54, `Version skew` job 03:53:06 → 04:01:33 (**8 min 27 s**;
the suite itself 411 s), conclusion **failure**. Every number and quote below
is from the job log (saved in the session scratch as `run35-job.log`).

| test | predicted | actual | time | what the log shows |
|---|---|---|---|---|
| `TestPinnedPeer/piri/sha-96a672e` | FAIL | **FAIL** | 121.6 s | `provenance verified: pinned=piri@…sha-96a672e head=hilt,ingot,upload`; login succeeded; `upload: command failed with exit code 1` — guppy's `/blob/add` got a readable receipt from HEAD sprue with `{"name": "CandidateUnavailable", "message": "no storage providers available"}`; HEAD sprue's log: `/blob/allocate` to `http://piri-0:3000` → `executing request: missing receipt for task: bafyreie7zcd…`, `failed to allocate blob`, `no candidates available after filters applied`; the pinned piri-0 logged `/blob/allocate space: …`, `allocated piece`, `request completed {"status": 200 …}` — it answered, and HEAD sprue could not read the answer |
| `TestPinnedPeer/ingot/sha-96a672e` | PASS | **PASS** | 133.1 s | `compat: exercised shape=pinned-peer pinned=ingot@…sha-96a672e head=hilt,piri,upload` — the run's only exercised configuration |
| `TestRollingUpgrade/upgrade_piri` | FAIL | **FAIL** | 51.7 s | `provenance verified: pinned=hilt@…sha-f60dd59,ingot@…sha-f60dd59,upload@…sha-f60dd59 head=piri`; `login: command failed` — guppy (`user_agent: guppy/main-unknown`) `/access/request` → old sprue logged `requesting access` and `request completed {"status": 200 … "response_size": 413}`; guppy: `missing receipt for task: bafyreig5yvq7…`. HEAD piri was never contacted |
| `TestRollingUpgrade/upgrade_ingot` | FAIL | **FAIL** | 50.1 s | identical shape, `missing receipt for task: bafyreidrd234…` at login |
| `upgrade_sprue`, `upgrade_hilt` | FAIL (predicted) | **not run** | — | prediction-table error: `TestRollingUpgrade` upgrades only the operator-run services (`piri`, `ingot`); sprue and hilt are pinned peers, never the upgraded one |
| `TestProvenanceGuardFires` | PASS | **PASS** | 54.9 s | fired on the bind-mount branch: `piri-0: pinned to ghcr.io/fil-forge/forge/piri:sha-96a672e but a binary is bind-mounted over /usr/bin/piri (from /tmp/TestProvenanceGuardFires…/piri) — the pin is a label and HEAD is what runs` |
| `TestCheckProvenance` (12), `TestDescribe`, `TestBindMountAt`, `TestServiceTable` | PASS | **PASS** | 0.00 s | |
| `Refuse a vacuous pass` | passes | **skipped** | | the step runs only after a successful suite step; a failing suite is not a vacuous pass, so nothing was lost, but the gate was never evaluated (latent bug 20) |
| job conclusion | failure | **failure** | | |

Six of six predictions that named a configuration the suite runs were right,
including the two passes; the two wrong rows named configurations that do not
exist. The D agent's earlier "What to expect" table — written for `f60dd59`'s
dependencies — had `TestPinnedPeer/piri` as *expected: pass*; on this branch it
fails for the reason given under "Why the predictions above no longer apply".

**What failed, mechanically.** Every failure is a HEAD-built UCAN client
(HEAD sprue talking to the `sha-96a672e` piri; today's guppy talking to the
`sha-f60dd59` sprue) receiving an HTTP 200 whose receipt it cannot see. The
message is two layers away from the cause: ucantone's
`container.decodeTokens` tries `receipt.Decode` on each token, and post-#49
that returns `invalid receipt, audience must be omitted` for a receipt issued
by a pre-#49 executor; the error is discarded, the token is not counted as a
receipt, and `client.Execute` reports `missing receipt for task`. The string
`audience must be omitted` appears nowhere in the log. Servers logged nothing
wrong (latent bug 19).

**What the run validated about the mechanics** — all of it for the first
time on a runner: override inputs reach the suite (`COMPAT_PIRI_VERSIONS`,
`COMPAT_BASELINE_*`, `COMPAT_EXPECT_PIN_MISMATCH=1` in the step env); the
pre-pull fetched all six refs (`Status: Downloaded newer image` each) in 11 s;
`pkg/workspace` built hilt, ingot, upload and piri from the working tree on
the runner and bind-mounted them; every stack came up healthy (`healthcheck`
hits all `200`; no compose, pull, CLI or `serve serve` error anywhere); the
provenance guard found the right containers, passed in all four real stacks
and fired in the negative test; teardown was clean. Time to a ready stack:
about 108 s for the first (including the first HEAD builds of three
services), about 119 s for the second (the first piri build), about 40 s for
the two baseline stacks whose only HEAD build was cached — versus the 20–40
minute estimate above. Old images under HEAD's compose ran fine (latent bug
9's `serve serve` was tolerated, as predicted).

**Reading.** On its first real execution the suite caught a one-directional
wire break between HEAD and the images the network was running, on a Go API
that had "held" (Experiment A: zero compile errors, all unit tests green).
It is the same break Experiment E classifies as August's one real breaking
library change (`bfc05d9`) and dates to the 08-20/21 straddle in the live
fleet; here it is reproduced in a lab, attributed to a component pair, and
would have been red on the nightly every day since the bump — had the nightly
been able to run.

### Run 36 — same pins, guppy pinned to its 2026-07-31 build, no guard

[33834923448](https://github.com/fil-forge/forge/actions/runs/33834923448):
dispatched 03:55:11 and queued behind run 35; `Version skew` job 04:01:42 →
04:09:12 (**7 min 30 s**; the suite 358 s), conclusion **failure**. The
pre-pull fetched seven refs in 13 s (the six above plus
`ghcr.io/fil-forge/guppy:sha-c94d43b-dev`, digest `sha256:f0a12af0…`); every
stack logged `compat: guppy client pinned to ghcr.io/fil-forge/guppy:sha-c94d43b-dev`
and `main-dev` appears nowhere in the log. Log saved as `run36-job.log`.

| test | predicted | actual | time | what the log shows |
|---|---|---|---|---|
| `TestPinnedPeer/piri/sha-96a672e` | FAIL | **FAIL** | 117.0 s | the skew without guppy in the loop: HEAD sprue's `/blob/allocate` to the pinned piri → `executing request: missing receipt for task: bafyreibc2il…`, `failed to allocate blob`; piri-0 logged `allocated piece` and a 200. The 10 MB shard never reached piri: `no candidates available after filters applied {… "total": 1}` — whether that filter is a consequence of the failed allocate cannot be read from the log |
| `TestPinnedPeer/ingot/sha-96a672e` | PASS | **FAIL** | 134.7 s | **the July guppy vs HEAD sprue**: guppy's `/ucan/conclude` (arguments `{"receipt": {"/": "bafyreiff7vtl…"}}`, its `/http/put` receipt attached) → HEAD sprue `concluding received receipt` then `receipt not found in invocation metadata` → guppy `conclusion receipt not found`; HEAD piri then logged the aborted shard PUT as `writing file: unexpected EOF` (400). The pinned ingot is not implicated: nothing on the path touches it |
| `TestRollingUpgrade/upgrade_piri` | PASS | **PASS** | 54.6 s | `compat: exercised shape=rolling-upgrade pinned=hilt@…sha-f60dd59,ingot@…sha-f60dd59,upload@…sha-f60dd59 head=piri` — the only exercised configuration of the run, and the only one in either run where an old component read a new one's receipts: the `sha-f60dd59` sprue allocated and accepted against HEAD piri, and upload and retrieve completed |
| `TestRollingUpgrade/upgrade_ingot` | PASS | **FAIL** | 52.1 s | **the floating indexer vs the old piri**: both blobs uploaded and accepted (`/blob/accept`, PUTs 204); `/index/add` → old sprue → indexer `/assert/index` → the indexer fetched the index blob from piri-0 by `/content/retrieve` (piri: `content/retrieve result {"status": 200, "content_length": "1292"}`) and reported `missing receipt for task: bafyreibe6fgf…`; guppy: `Upload failed with non-retriable error` |
| `TestProvenanceGuardFires` | skipped | **skipped** | 0 s | `COMPAT_EXPECT_PIN_MISMATCH unset` |
| `TestCheckProvenance` (12), `TestDescribe`, `TestBindMountAt`, `TestServiceTable` | PASS | **PASS** | 0.00 s | |
| job conclusion | failure | **failure** | | |

Four of six right. Both misses are the same receipt rule reaching a component
pair the predictions did not consider:

- **guppy `c94d43b` ↔ HEAD sprue.** The pre-#49 guppy issues its `/http/put`
  receipt with `aud`. HEAD sprue's conclude handler
  (`sprue/pkg/service/handlers/ucan_conclude.go`) looks for that receipt among
  `req.Metadata().Receipts()`, which post-#49 `container.decodeTokens` never
  populates with an `aud`-bearing token — so "not found". The same guppy's
  conclude against the `sha-f60dd59` sprue in `upgrade_ingot` went through
  (`found invocation for conclusion`, `accept success`). The decode step is
  read from the code; the rest is in the log.
- **`indexing-service:main` ↔ the `sha-f60dd59` piri.**
  `smelt/systems/indexing/indexer/compose.yml` runs
  `${INDEXER_IMAGE:-ghcr.io/fil-forge/indexing-service:main}` — floating;
  `indexing-service` `main` pins ucantone `3a20cd5` (2026-08-17, post-#49) —
  so today's indexer cannot read a pre-#49 piri's retrieval receipt. With HEAD
  piri as the writer (`upgrade_piri`) the same path passed. The prediction
  table treated guppy as the only floating component; the indexer, the
  delegator (`delegator:main`) and the signing service
  (`piri-signing-service:main`) float too (latent bug 21).

Provenance was verified in all four stacks; boot-to-failure inside each
stack was 38–39 s; the two pinned-peer subtests spent about 80–95 s more
building three HEAD services each, the rolling-upgrade subtests built one.

### Reading both runs together

- **One asymmetric library change accounts for all seven failing stacks.**
  ucantone `bfc05d9` (#49): a post-#49 component cannot read a receipt issued
  by a pre-#49 executor; the reverse works. Seen through four component pairs
  — HEAD sprue ↔ old piri (both runs), today's guppy ↔ old sprue (run 35),
  July's guppy ↔ HEAD sprue (run 36), today's indexer ↔ old piri (run 36) —
  and never in the other direction: the one configuration in which an old
  reader faced a new writer (old sprue reading HEAD piri in `upgrade_piri`,
  run 36) passed. Both passing stacks are the ones in which every reader is
  at least as new as its writer.
- **The rollout order this implies** is executors before their clients:
  piri, then sprue, then indexer, guppy and anything else that reads
  receipts. Experiment E's reading of the 08-20/21 straddle had the risk on
  the wrong side (ingot, guppy and delegator staying old was the safe side);
  corrected there.
- **The Go API hid all of it.** Experiment A bumped ucantone with zero compile
  errors and green unit tests; nothing short of booting old images against
  HEAD showed the break. That is the suite's purpose, and it did it on its
  first execution.
- **The suite is not hermetic.** Four of the stack's components float
  (`guppy:main-dev`, `indexing-service:main`, `delegator:main`,
  `piri-signing-service:main`); the two runs differ by one of them and
  disagree on `TestPinnedPeer/ingot` and `upgrade_ingot` because of it. A
  compat verdict today depends on the day's images of three repositories
  outside forge — the polyrepo shows up inside the monorepo's own test
  harness.
- **The error message will cost someone a day.** Every failure surfaced as
  `missing receipt for task` or `conclusion receipt not found` against a
  server that logged a 200. `audience must be omitted` appears in neither
  log (latent bug 19).

## Not done, and reversibility

- The D agent dispatched nothing (brief rule 5); the coordinator dispatched
  runs 35 and 36 on the POC branch, recorded above. No tag was created; no
  push to `main`.
- No Docker daemon in the build environment, so `TestPinnedPeer`,
  `TestRollingUpgrade` and `TestProvenanceGuardFires` ran for the first time
  on the GitHub runner; the `TestBuild*` tests in `pkg/stack` still have not
  run in this session.
- `pkg/stack` gained two exported methods and one new error path; `pkg/workspace`
  two exported functions. Additive; downstream users of the smelt SDK are
  unaffected unless they were passing an unknown exclusion name.
- Everything is on `poc/d-compat`; reverting is `git branch -D`.
