# Is `guppy` functional against the current stack? (plan question 6)

Investigation date: 2026-09-03. All commands were run read-only against the
checkouts listed in `POC-CONTEXT.md` (`/home/user/guppy` main `e87812b`,
`/home/user/ucantone` main `8d7eb73`, `/home/user/libforge`,
`/home/user/forge`, `/home/user/fil-forge/*`) plus anonymous GHCR and the
GitHub Actions API for `fil-forge/guppy` and `fil-forge/forge`. Raw GHCR
responses are saved under the session scratch directory (not committed) (`*.idx.json`, `*.man.json`,
`tags.json`, `*.hdr`).

## Fact table

| # | Fact | Evidence |
|---|---|---|
| G1 | `ghcr.io/fil-forge/guppy:main-dev` is guppy commit **`d74fd06`** ("chore: update deps (#47)", authored 2026-08-21 08:38 +0200 by alanshaw). | The `main-dev` OCI index digest `sha256:4d8950d8…9d93d` is byte-identical to the digest of tag `sha-d74fd06-dev`; likewise `main` == `sha-d74fd06` (`sha256:ea32605b…6bd8822`). Digests from `Docker-Content-Digest` headers in `inv-guppy/main-dev.hdr`, `sha-d74fd06-dev.hdr`, `main.hdr`, `sha-d74fd06.hdr`. `publish-ghcr.yml` tags each main push with `main-dev` **and** `sha-<short>-dev` from the same build, so equal digests mean same build. |
| G2 | `main-dev` was built by guppy Container run 32455216173 on **2026-08-21 06:38:30Z–06:46:14Z** (conclusion `success`). amd64 manifest `sha256:3a7c6667…57cff`, config blob `sha256:4b011568…21c4f`. | `mcp__github__actions_get get_workflow_run 32455216173` → `created_at 2026-08-21T06:38:30Z`, `updated_at 06:46:14Z`, `head_sha d74fd060…`. |
| G3 | OCI config labels (`org.opencontainers.image.revision/.created`) could **not** be read: GHCR redirects blob downloads to `pkg-containers.githubusercontent.com`, which the egress proxy rejects (`CONNECT tunnel failed, response 403`, `connect_rejected (organization policy)`). G1/G2 rest on digest equality + the workflow run instead, which is equivalent evidence. | curl output in this session; 9 blob fetches all rejected. |
| G4 | The guppy image tag list has 28 tags: `main`, `main-dev` and 13 `sha-*`/`sha-*-dev` pairs: d33e659, 5036e84, 11f8a6c, 6bb73e3, 870419f, 547c636, 729ecee, 6b7ece0, 4cd2b6c, ce38bc1, c95e05c, c94d43b, d74fd06. **Newest image is d74fd06; guppy main HEAD `e87812b` (and `4734499`, `08f5554`) were never published.** | `inv-guppy/tags.json` (`/v2/fil-forge/guppy/tags/list?n=1000`). |
| G5 | Why 25 of 40 main commits have no image and no main CI run: they are Dependabot PRs squash-merged by `dependabot-auto-merge.yml` using `secrets.GITHUB_TOKEN` (`gh pr merge --squash`). Pushes made with `GITHUB_TOKEN` do not fire `push` workflows, so `Test`, `Check` and `Container` never ran on main for them. The Aug-1 Dependabot commits that *do* have images (`ce38bc1`, `c95e05c`, `c94d43b`) were merged by a human (`actor: hannahhoward` on their runs). | Workflow-run listing for `fil-forge/guppy` branch `main` (79 runs): after `d74fd06` only `dynamic/dependabot/dependabot-updates` runs exist for heads `4734499`; none for `08f5554`/`e87812b`. `git log --format='%an %cn'`: all missing commits `author=dependabot[bot] committer=GitHub`. |
| G6 | On 2026-07-31, when forge CI at `f60dd59` ran green (run 30673321899, `created_at 2026-07-31T23:35:47Z`, finished 23:48:52Z), `guppy:main-dev` was **`6b7ece0`** ("ci: add cross-repo workspace test caller (#18)"). Its Container run 30657336866 finished 2026-07-31T19:18:50Z (success); the next successful Container run (`ce38bc1`, run 30687103165) finished 2026-08-01T06:18:52Z, after the forge run. `4cd2b6c`'s Container run was `cancelled` (concurrency) and `1f93dd8`'s too. | Run listing page 2 for `fil-forge/guppy`; `mcp__github__actions_get 30673321899` for forge. |
| G7 | guppy pins at `6b7ece0` (the image the green run used): `libforge v0.0.0-20260630210927-2b55dbcf944f`, `ucantone v0.0.0-20260619013642-7985ec010b88`. forge at `f60dd59` pinned `libforge …928cf2a21b7e`, `ucantone …ccb77059de44` (F2). Both sides pre-date ucantone #49 → compatible; the run was green. | `git show 6b7ece0:go.mod`; `git show f60dd59:sprue/go.mod` etc. |
| G8 | guppy main (`e87812b`, and the image `d74fd06`) pins `libforge v0.0.0-20260807225550-3e6895b41be5` (2026-08-07) and `ucantone v0.0.0-20260817170631-3a20cd59fabc` (2026-08-17), set by `d74fd06` (diff: `2b55dbc→3e6895b`, `7985ec0→3a20cd5`). Also `indexing-service v1.13.5-0.20260619142411-efe3f5fab717`. | `git show d74fd06 -- go.mod`. |
| G9 | `3a20cd5` **does include** ucantone #49 (`bfc05d9`, receipt spec alignment) and #50 (`e926fd5`, issued-at/receipt timestamps); it does **not** include #52 (codegen refactor), #53 (validator fix), #37 (ML-DSA-44), #55 (container transport decode). `3a20cd5` itself is #43 ("explicit empty capabilityInvocation… endorse nothing"), the last commit of 2026-08-17. | `git merge-base --is-ancestor` for each PR commit vs `3a20cd5`. |
| G10 | Live services' pins (F17): piri, sprue, piri-signing-service → libforge `3e6895b` (**identical to guppy's**); hilt, ingot → libforge `c9252ac` (PR #64 head); ucantone: piri/sprue/hilt/ingot/delegator `25cf834` (post-#49), piri-signing-service `3a20cd5` (same as guppy), smelt `79141c5` (pre-#49, but smelt is only the orchestrator). libforge `3e6895b→c9252ac` adds only `Tenant did.DID` to `commands/s3/request.AuthorizeOK` (S3/hilt path, not used by guppy) plus dep bumps. | `git log 3e6895b..c9252ac`, `git show 2585ed1 -- commands/s3/request/types.go`. |
| G11 | Coordinator's forge branch pins libforge `f4b13f7` (main + PR #52) and ucantone `8d7eb73` (post-#49) in all five modules → receipt-compatible with guppy `main-dev`. | `grep fil-forge */go.mod` in `/home/user/forge`. |
| G12 | **ucantone #49 is a wire-visible asymmetric break.** Pre-#49 executors issue receipts with `aud` set (`receipt.go:217 invocation.WithAudience(executor.DID())` at `ccb7705`). Post-#49 `receipt.fromInvocation` returns `"invalid receipt, audience must be omitted"` when `inv.Audience().Defined()`. In `container.decodeTokens` (3a20cd5 `ucan/container/container.go:327`) a failed `receipt.Decode` falls through to `invocation.Decode`, so the receipt is silently classified as an invocation; `client.Execute` (3a20cd5 `client/client.go:84-91`) then fails with `missing receipt for task: <cid>`. guppy uses exactly this path (`pkg/client/client.go:138-144` `executor.Execute` → `resp.Receipt()`; `blobadd.go:253` `receiptsClient.Poll` → libforge `receipt.Client.Fetch` → `ct.Receipt(task)` → `"receipt not found in UCAN container"`). Old client + new executor is fine (old decoder never checked `aud`). | Code cited; `git show bfc05d9`. |
| G13 | Consequence: **forge `main` at `f60dd59` (ucantone `ccb7705`) + today's floating `guppy:main-dev` (`d74fd06`, ucantone `3a20cd5`) is predicted to fail the smoke path at the first `blob/add` receipt.** This has not been observed in CI because forge has had no push since 2026-08-01 (F8) and `compat.yml` never boots a stack (F9). Not executed here (no Docker daemon) — **prediction, flagged as such**, but it follows directly from G12 and the pins. The POC branch (G11) is not affected. | G7, G11, G12; `smelt/systems/guppy/compose.yml` at `f60dd59` and on the POC branch both use `${GUPPY_IMAGE:-ghcr.io/fil-forge/guppy:main-dev}`; `ci.yml` never sets `GUPPY_IMAGE`. |
| G14 | guppy's own CI on main: every `Test`/`Check`/`Container` push run in the 79-run history is `success` (or `cancelled` by concurrency: Container for `1f93dd8`, `4cd2b6c`, `c95e05c`, `147670a`). Last real main run: `d74fd06` 2026-08-21 (Test 32455216155, Check 32455216158, Container 32455216173, all success). `Test` = `go build ./... && go test -v -cover ./...`; `Check` = tidy diff, vet, staticcheck, gofmt. No e2e in guppy's CI. | Run listing pages 1–2. |
| G15 | Dependabot's own `go_modules` update job fails on every weekly run (`dependabot-updates` runs 33159972878, 32468475129, 31788637997, 31166529259, 30657309294 all `failure`): `github.com/libp2p/go-libp2p` `dependency_file_not_resolvable` (it tries `v6.0.23+incompatible`). Individual per-dependency PRs still get created. Cosmetic, but it is the only red on guppy's main. | `get_job_logs 33159972878 failed_only`. |
| G16 | Standalone build of guppy main (`GOWORK=off GOFLAGS=-mod=readonly go build ./...` + `go vet ./pkg/client/... ./pkg/tokenstore/...`): see "Build check" below. | this session |
| G17 | What `smelt/pkg/clients/guppy` exercises (via `docker exec` into the `guppy` container whose entrypoint is `sleep infinity`): `guppy login <email>` (raced against an smtp4dev validator that POSTs the validation link from inside the network), `guppy space generate`, `randdir --size 10MB` (dev-image tool, not guppy), `guppy upload source add <space> <path>`, `guppy upload [--replicas N] <space>`, `guppy retrieve <space> <cid> <dest>`. `smoke_test.go` runs this for `postgres` and `s3_and_postgres` permutations with `WithReplicas(1)`; `snapshot_test.go` does login/space/100MB upload (no retrieve); `tests/compat/compat_test.go::assertUploadRetrieve` mirrors the smoke path; `systems/stress-tester/internal/guppy/cli.go` drives the same five commands. Retrieve output is not compared to the uploaded bytes (only exit status). | `smelt/pkg/clients/guppy/container.go`, `tests/e2e/smoke_test.go`, `tests/e2e/snapshot_test.go`, `tests/compat/compat_test.go:202-231`. |
| G18 | guppy commands/capabilities **not** exercised by any smelt path: `account list`, `blob ls`, `delegation create`, `gateway serve`, `identity generate`, `proof add`, `space info/list/provision`, `unixfs ls`, `upload check`, `upload demo`, `ls`, `reset`, `verify`, `whoami`, `version`; the PostgreSQL sqlrepo backend; resumption of interrupted uploads; multi-source spaces. Also untested: guppy's `retrieve` correctness (bytes not compared). | `ls /home/user/guppy/cmd/*` vs G17. |
| G19 | guppy's config in the stack (`smelt/systems/guppy/config/guppy-config.toml`): `upload_id did:web:upload`, `upload_url http://upload:80`, `receipts_url http://upload:80/receipt`, `indexer_id did:web:indexer`, `indexer_url http://indexer:80`, `authorized_retrievals=true`, `insecure_did_resolution=true`, key from `/keys/guppy.pem`. | file cited. |
| G20 | Nothing in guppy declares itself deprecated/unmaintained: `grep -i "deprecat|unmaintain|archived|no longer|not maintained|experimental"` over `README.md`, `AGENTS.md`, `CONTRIBUTING.md`, `docs/content/*.md` returns nothing. README still brands it "Storacha's go uploader" and links `storacha.github.io/guppy`; `version.json` is `v0.7.0`; CODEOWNERS `@hannahhoward @Peeja @alanshaw @volmedo`. 7 open issues, all from 2026-05-18/29 (#2–#7, #14: `up.web3.storage` string, atomicity nits, agent-store perms, gateway shutdown ctx, optional `key_file`) — none about status. Last non-dependency human commit: `d74fd06` (2026-08-21, alanshaw dep bump); last feature/fix: `547c636` 2026-07-01 ("deps: update libforge w/ login fix"), `6bb73e3` 2026-06-19 ("fix: do not send allocate or accept delegations"). All 15 most-recently-updated PRs are Dependabot except #47. | `mcp__github__list_issues`, `mcp__github__list_pull_requests`, `git log`. |
| G21 | The forge stack's `guppy` service is the only external image the stack depends on that floats (`main-dev`); piri/hilt/sprue/ingot are built from the PR tree. | `ci.yml` stack job env sets `PIRI_IMAGE`/`HILT_IMAGE`/… but no `GUPPY_IMAGE`; compose default `ghcr.io/fil-forge/guppy:main-dev`. |

## Build check (G16)

`cd /home/user/guppy && GOWORK=off GOFLAGS=-mod=readonly go build ./... && go vet ./pkg/client/... ./pkg/tokenstore/...`
at `e87812b` (guppy main HEAD): **build OK, vet clean**, exit 0, `real 1m22s`
(cold module download included). `git status` afterwards is empty, the
checkout was not modified. This is the commit that guppy's CI never ran on
(G5); it compiles against `libforge 3e6895b` + `ucantone 3a20cd5`.

## Answer

**Yes for the smoke path, with one important qualifier about *which* pair of
versions.** The evidence that guppy works: (1) forge CI at `f60dd59` drove
`login → space generate → upload source add → upload --replicas 1 → retrieve`
through `guppy:main-dev` = `6b7ece0` and was green on 2026-07-31 (G6, G7,
F11); (2) guppy `main-dev` today is `d74fd06` (2026-08-21) and pins **exactly
the libforge revision the live piri/sprue run** (`3e6895b`) and a ucantone
revision (`3a20cd5`) that is on the same side of the receipt-spec change (#49)
as every live service (`25cf834`) and as the POC branch (`8d7eb73`) (G8–G11);
(3) guppy's own unit CI is green at its last real main run and the module
builds standalone (G14, G16); (4) nothing in the repo or its issues declares
it deprecated (G20). The evidence against is weaker than the "non-functional"
description suggests: guppy is *idle*, not broken — no feature commit since
July, main HEAD `e87812b` was never CI-tested or published because
`GITHUB_TOKEN` auto-merges do not trigger workflows (G4, G5), and the only
red is Dependabot's own resolver (G15). The qualifier: ucantone #49 makes
new-guppy-vs-old-executor incompatible (`missing receipt for task`, G12), so
**forge `main` as it stands (`f60dd59`, ucantone `ccb7705`) plus the floating
`main-dev` image would, by code reading, fail the smoke path today** (G13 —
a prediction, not observed; nothing has booted that combination since
2026-07-31 and `compat.yml` never boots anything). This is the concrete case
for pinning `GUPPY_IMAGE` to a `sha-*-dev` tag in `ci.yml`/`compat.yml`
rather than floating `main-dev`, and it is an example of exactly the
wire-visible library change Experiment E is meant to count. What the smoke
path does *not* tell you (G17, G18): whether retrieved bytes match, whether
resumable/large uploads, Postgres mode, the gateway, delegations, proofs or
`verify` work; Alan's RFC statement "we want it to continue to work with
every version of the network" is a stronger claim than anything CI checks.
Speculation, flagged: the "non-functional" description may date from the
June–July window when guppy's pins (`7985ec0`/`2b55dbc`) lagged the services
by 6–8 weeks; the 2026-08-21 dep bump (#47) closed that gap.
