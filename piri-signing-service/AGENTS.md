# AGENTS.md — piri-signing-service

Guidance for engineers and AI agents working in this repo. The code is ground
truth; if this file and the code disagree, follow the code and fix this file.

## Purpose

Go HTTP service that wraps a cold wallet to produce EIP-712 signatures for PDP
(Proof of Data Possession) operations on Filecoin. Piri storage nodes request
signatures from this service instead of holding the operator's private key.
The signing key never leaves the service; the verifying contract is a
FilecoinWarmStorageService deployment.

Current security posture is Phase 1 "blind signing" on the legacy REST routes
(no authentication — do not expose to the public internet), with UCAN 1.0
authenticated invocations as the primary path going forward.

## Build / Test / Run

```bash
make build          # go build -o signer . (binary is named `signer`)
make test           # go test -v ./...
make test-coverage  # coverage.out + coverage.html
make lint           # golangci-lint run
make run            # build + run
make fmt / tidy / install / clean
```

Module: `github.com/fil-forge/forge/piri-signing-service`, Go 1.25.

## Layout

```
main.go                  # Cobra CLI entry point: config, key loading, Echo server, routes
pkg/
  config/               # Viper config: flags > SIGNING_SERVICE_* env > signer.yaml (cwd) > defaults
  signer/               # Core EIP-712 signer: SignCreateDataSet, SignAddPieces,
                        #   SignSchedulePieceRemovals, SignDeleteDataSet, Recover* helpers
  types/                # SigningService + OperationSigner interfaces, legacy request/response types
  handlers/             # Legacy REST handlers (/sign/* and /healthcheck)
  server/               # UCAN 1.0 HTTP server construction (ucantone), DID resolvers
    handlers/           # UCAN invocation handlers (access grant + the four sign commands)
  inprocess/            # In-process SigningService impl (no network, no auth checks) — for tests/dev
  client/               # Go client library: UCAN invocations against a remote signing service
deploy/                 # Terraform/OpenTofu (generated with storoku); GH Actions deploys on release
signer.yaml.example     # Full config template
```

## Configuration

Priority: CLI flags > env vars (`SIGNING_SERVICE_` prefix) > `signer.yaml` in
the working directory > defaults (host `localhost`, port `7446` — "SIGN" on a
T9 keypad).

Required:
- `rpc_url` — Ethereum RPC endpoint (chain ID is detected from it at startup)
- `service_contract_address` — FilecoinWarmStorageService contract
- ECDSA signing key: one of `signing_key` (hex), `signing_key_path`, or
  `signing_keystore_path` + `signing_keystore_password`
- Service identity (UCAN issuer): `service_key` (multibase Ed25519) or
  `service_key_file` (Ed25519 PEM) — exactly one, not both — plus `service_did`
  (must be a did:web), which is required whenever a service key is set

Dev-only hidden flag: `--insecure-did-resolution` enables did:web resolution
over plain HTTP (used in tests/local setups).

Deployment note: non-secret production env vars (`SIGNING_SERVICE_RPC_URL`,
`SIGNING_SERVICE_SERVICE_CONTRACT_ADDRESS`) live in
`deploy/.env.production.local.tpl`.

## HTTP API

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/` | UCAN invocation endpoint (primary; ucantone `server.HTTPServer` wrapped as an Echo handler) |
| GET | `/healthcheck` | Health: signer address, chain ID |
| POST | `/sign/create-dataset` | Legacy blind-sign CreateDataSet |
| POST | `/sign/add-pieces` | Legacy blind-sign AddPieces |
| POST | `/sign/schedule-piece-removals` | Legacy blind-sign SchedulePieceRemovals |
| POST | `/sign/delete-dataset` | Legacy blind-sign DeleteDataSet |

The `/sign/*` routes are marked TODO-remove once all piri nodes have moved to
UCAN invocations — assume they may still be in use; don't remove or change
their wire format without checking deployed piri nodes.

## UCAN Commands

The service uses UCAN 1.0 via `github.com/fil-forge/ucantone` with command and
argument types defined in `github.com/fil-forge/forge/protocol` (`commands/access`,
`commands/pdp/sign`). Five commands are handled (`pkg/server/server.go`):

| Command | Handler | Notes |
|---|---|---|
| `/access/grant` | `pkg/server/handlers/access_grant.go` | Issues 1-hour delegations, one per requested attenuation; only `/pdp/sign/*` commands are grantable; delegations attached to the response container, CIDs in the receipt OK |
| `/pdp/sign/dataset/create` | `pkg/server/handlers/sign.go` | |
| `/pdp/sign/dataset/delete` | `pkg/server/handlers/sign.go` | |
| `/pdp/sign/pieces/add` | `pkg/server/handlers/sign.go` | |
| `/pdp/sign/pieces/remove/schedule` | `pkg/server/handlers/sign.go` | |

Sign handlers require the invocation subject to equal the service DID
(i.e. the caller must hold a delegation from the service — self-signed
invocations fail with an `InvalidResource` receipt error).

Piri clients sign invocations as did:web identities, so the server registers a
did:web resolver (HTTPS did.json fetch, 3-hour cache; the service's own DID is
resolved from a well-known in-memory document). Without this the validator
only accepts did:key issuers.

## Key Interfaces

`pkg/types` defines two interfaces — keep them in sync when adding operations:

- `OperationSigner` — the raw EIP-712 signing surface (`pkg/signer.Signer`
  implements it; used by both legacy handlers and UCAN handlers)
- `SigningService` — the higher-level authorized-signing surface. Implemented
  by `pkg/client.Client` (remote, UCAN invocations) and `pkg/inprocess.Signer`
  (direct, no auth — testing/dev). Piri consumes this interface, so changes
  here are breaking for piri.

## Key Dependencies

- `github.com/ethereum/go-ethereum` — ECDSA keys, keystore, RPC client
- `github.com/fil-forge/filecoin-services/go` — `eip712` typed-data signing utilities
- `github.com/fil-forge/ucantone` — UCAN 1.0 (server, client, delegation, DID resolvers)
- `github.com/fil-forge/forge/protocol` and `github.com/fil-forge/forge/internal` — shared UCAN command/argument schemas, service identity
- `github.com/labstack/echo/v4` — HTTP server; `spf13/cobra` + `viper` — CLI/config

## Conventions

- Testing: standard `go test` with testify (`assert`/`require`); tests live
  next to the code (`*_test.go`)
- Logging: `ipfs/go-log/v2` in packages, Echo middleware logger for HTTP
- Go file naming: snake_case

## Blast Radius / Gotchas

- Changing EIP-712 message construction (`pkg/signer`, or the eip712 types it
  uses) breaks on-chain PDP proof verification — the contract must recover
  the expected signer from the exact typed data.
- The UCAN command/argument schemas are defined in the in-repo `protocol` module, not here.
  Changing request/response shapes means coordinating a `protocol` change and
  updating piri (the client side) as well.
- Key-loading changes affect every piri node's ability to obtain valid
  signatures.
- Chain ID is baked into signatures via the EIP-712 domain; a wrong `rpc_url`
  (wrong network) yields signatures the contract rejects.
