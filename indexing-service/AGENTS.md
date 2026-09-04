# AGENTS.md — indexing-service

Guidance for engineers and AI agents working in this repo.

## Purpose

Go service for content routing on the Forge network: a cache-and-query node that
bridges IPNI (InterPlanetary Network Indexer) with UCAN content claims. Clients and
gateways query it to find which storage nodes hold a given CID/multihash; storage and
upload services publish claims into it. Runs as a standalone HTTP server, deployed as
a container (ECS via the storoku Terraform module, port 8080). Lambda entry points
still exist under `cmd/lambda/` from the earlier Lambda/API-Gateway architecture, but
that is not how the service is deployed anymore.

## Quick reference

```bash
make indexer            # go build -o ./indexer ./cmd
make test               # go test -race -v ./...
make test-nocache       # clear test cache, then test
make mockery            # regenerate mocks (.mockery.yaml, testify template)
make gen                # go generate ./... (cbor/json codegen, e.g. pkg/service/queryresult)
make ucankey            # generate a multibase ed25519 private key for dev deployments
                        # (currently broken: depends on a `ucangen` binary with no build rule in-repo)

go run ./cmd server     # start standalone HTTP server
go run ./cmd aws        # run as the containerized server used in AWS deployments (not a local Lambda emulator)
./indexer query <CID>   # query an indexer node (default: https://indexer.forge.fil.one)
./indexer query -u <URL> <CID>   # query a specific node
```

## Layout

```
cmd/
  main.go               # CLI entry point: server | aws | query subcommands
  server.go, aws.go, query.go
  lambda/               # one main per Lambda: getroot, getclaim, getclaims,
                        # getdiddocument, postclaims, notifier, providercache, remotesync
pkg/
  server/server.go      # HTTP mux, all routes, gzip + OTel wrapping
  service/
    service.go          # core query/publish/cache orchestration
    contentclaims/      # claim stores/finders + UCAN server (ucanserver.go, ucanservice.go)
    providerindex/      # IPNI provider index (+ remotesyncer/)
    blobindexlookup/    # sharded DAG index lookup
    providercacher/     # SQS-backed provider caching queue
    queryresult/        # query result encoding (cbor_gen.go/json_gen.go are generated)
  construct/construct.go # dependency injection: Construct(ServiceConfig, ...Option)
  types/types.go        # core interfaces: Store/Cache generics, Getter, Publisher, Querier, Service
  aws/                  # DynamoDB/S3/SQS/SNS adapters for the Lambda deployment
  redis/                # Redis-backed stores (provider, claims, no-provider, sharded DAG index)
  presets/              # preset endpoint/config values
  telemetry/            # OpenTelemetry helpers
  internal/jobwalker/   # parallel/single job walking used by query fan-out
deploy/                 # OpenTofu/Terraform (app/ + shared/), invoked via deploy/Makefile
docs/infra.md           # AWS infrastructure diagram (mermaid) — STALE: shows the old
                        # Lambda architecture; deploy/app/main.tf is the source of truth
```

## HTTP routes (pkg/server/server.go)

| Route | Handler | Purpose |
|---|---|---|
| `GET /` | GetRootHandler | Service/version info |
| `GET /claim/{claim}` | GetClaimHandler | Fetch a single claim by root CID |
| `POST /` and `POST /claims` | PostClaimsHandler | UCAN invocations (both paths serve the same handler) |
| `GET /claims` | GetClaimsHandler (gzipped) | Query claims by multihash |
| `GET /.well-known/did.json` | GetDIDDocument | DID document |
| `GET /cid/{cid}` | GetIPNICIDHandler | IPNI lookup by CID (only registered when IPNI is configured) |

## UCAN stack and capabilities

- UCAN framework is **ucantone** (`github.com/fil-forge/ucantone`), not go-ucanto.
  The UCAN server is `server.NewHTTP(id, ...)` with route registration via
  `ucanServer.Handle(route.Command, route.Handler)` — see
  `pkg/service/contentclaims/ucanserver.go`.
- Capability (command) definitions come from the in-repo **protocol** module
  (`github.com/fil-forge/forge/protocol/commands/...`). Handlers bind typed
  arguments/results via `binding.Request`/`binding.Response` and
  `res.SetSuccess(...)` / `res.SetFailure(...)` — see `ucanservice.go`.

Capabilities handled:

- `assert/equals` (`protocol/commands/assert.Equals`) — publish an equals claim (originates from `filecoin/submit` handling)
- `assert/index` (`assert.Index`) — publish an index claim (originates from `index/add`)
- `claim/cache` (`protocol/commands/claim.Cache`) — cache a claim from a storage node,
  keyed to the provider peer derived from the invocation issuer's did:key

## Key integrations

- **IPNI**: `github.com/ipni/go-libipni` (find/model, maurl) plus
  `github.com/fil-forge/go-ipni-tools` (advertisement publishing, metadata).
- **AWS** (source of truth: `deploy/app/main.tf` + `deploy/app/legacyclaims.tf`):
  DynamoDB tables `metadata` and `chunk-links`, plus read-only data sources for legacy
  tables (claims, block index, allocations, store, blob registry); S3 buckets
  `provider-caching`, `ipni-store` (public), `notifier-head`, `claim-store`,
  `ipni-publisher`, plus the legacy claims bucket; SQS queues `provider-caching`
  (standard), `ipni-publisher` (FIFO, high throughput), `ipni-advertisement-publishing`
  (FIFO); Redis Serverless caches (providers, no-providers, indexes, claims); SSM
  Parameter Store for the private key. No SNS topics or EventBridge schedulers are
  provisioned (`topics = []`) — the diagram in `docs/infra.md` and parts of the README
  describe the retired Lambda architecture and are out of sync with the Terraform;
  trust the `.tf` files.
- **Telemetry**: OpenTelemetry (per-route `otelhttp` wrapping); Sentry for errors.
  Honeycomb supported via `HONEYCOMB_API_KEY`; disable with `TELEMETRY_DISABLED`.

## Conventions

- Go 1.25.x module: `github.com/fil-forge/forge/indexing-service`. Files snake_case.
- Interfaces live next to implementations (`interface.go`); mocks are generated by
  mockery into the same directory as `mock_<Interface>.go` (testify template,
  config in `.mockery.yaml`). Regenerate with `make mockery`.
- Tests: testify assertions, table/behavior style, `-race` always (via `make test`).
- Generated code: `cbor_gen.go` / `json_gen.go` files are produced by `make gen`
  (go generate) — edit the generators/types, not the generated files.
- Service wiring happens in `pkg/construct/construct.go` (`Construct(ServiceConfig, ...Option)`);
  add new dependencies there rather than constructing them ad hoc in handlers.

## Deployment

Terraform/OpenTofu under `deploy/` (storoku ECS app module), driven by
`deploy/Makefile` — run these from the `deploy/` directory, not the repo root
(the 25-line top-level Makefile only has build/test targets):

- `make init` / `make validate` / `make plan` / `make apply` (also `upgrade`,
  `console`, `clean`, `wait-deploy`)
- `make apply` builds a linux/arm64 image with `docker buildx`, pushes it to ECR,
  and applies the Terraform with that image tag — there is no Lambda build step.
- Config via `deploy/.env.terraform` (copy from `deploy/.env.terraform.tpl`):
  `TF_WORKSPACE` (your name; `prod`, `staging`, `warm-prod`, `warm-staging` are
  reserved), `TF_VAR_app=indexer`, `TF_VAR_private_key` (`make ucankey`),
  `TF_VAR_did`, `TF_VAR_allowed_account_id`, `TF_VAR_region`, optional
  `TF_VAR_network`, `IPNI_ENDPOINT`, `IPNI_FALLBACK_ENDPOINTS`,
  `IPNI_ANNOUNCE_URLS`, `IPNI_FORMAT_PEER_ID`/`IPNI_FORMAT_ENDPOINT`,
  telemetry vars, and `TF_VAR_use_prod_vars`. (The README still says to copy
  `.env.local` to `.env` — that file does not exist; the tpl is authoritative.)
- Runtime env for the container is rendered to `deploy/.env.production.local`
  from its `.tpl` via `esh`.
- AWS credentials/region come from the standard AWS config chain — check your
  profile before `make apply`.

Release flow: merges to `main` auto-deploy staging. Production releases are cut by
bumping `version.json` on a branch; merging it triggers tagging, binary builds, a
GitHub release, and a production Terraform deploy (see `.github/workflows/`).

## Blast radius / gotchas

- Claim format or schema changes affect content routing for the entire network —
  every client and gateway that queries this service. Capability schemas are defined
  in `protocol`; changing them requires coordinating all publishers and consumers of
  those commands across the network.
- IPNI advertisement changes affect content discoverability network-wide.
- `POST /` duplicating `POST /claims` is a deliberate temporary compatibility fix —
  keep both wired until clients migrate.
- The `GET /cid/{cid}` route only exists when an IPNI config is provided; don't
  assume it in tests or clients.
- Legacy stores (`legacy_claims`, `legacy_block_index` tables and the legacy claims
  bucket) still serve reads — be careful when touching claim lookup paths.
