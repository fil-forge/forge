# forge

The Forge network's services and the code they share, as one repository of
independent Go modules.

## Layout

| directory | module | what it is |
|---|---|---|
| `piri/` | `github.com/fil-forge/forge/piri` | storage node |
| `hilt/` | `…/forge/hilt` | tenancy, access keys and the S3 wire contract's authorizer |
| `sprue/` | `…/forge/sprue` | upload service |
| `ingot/` | `…/forge/ingot` | S3 gateway over the network |
| `delegator/` | `…/forge/delegator` | storage-provider onboarding and delegation issuance |
| `piri-signing-service/` | `…/forge/piri-signing-service` | EIP-712 signing oracle for PDP operations |
| `indexing-service/` | `…/forge/indexing-service` | content-routing index (IPNI + UCAN content claims) |
| `smelt/` | `…/forge/smelt` | the stack harness: boots the whole network in Docker; owns the e2e, S3 and compatibility suites |
| `protocol/` | `…/forge/protocol` | the wire contract: every UCAN command binding (`commands/**`), `blobindex`, `receipt`, `retrieval`, and the `bytemap`/`digestutil` helpers they need. Depends only on ucantone and third-party modules |
| `internal/` | `…/forge/internal` | helpers the services share that are not wire contract: `identity`, `jobqueue`, `piece`, `sigv4`, `s3perm`, `zapucan`, `ucanexec`, the `client/hilt` and `client/delegator` RPC clients, `pdpsigner`, `ipni/advertisement`. Importable only from this repository |
| `attestation/` | `…/forge/attestation` | the did:mailto attestation extension (`attestation`, `didmailto`, the `attest` command). Depends only on ucantone and multiformats |
| `forgeclient/` | `…/forge/forgeclient` | the Forge protocol client an agent writes through (`/blob/add`, `/ucan/conclude`, `/index/add`, … against the upload service), its `tokenstore`, and the indexer-backed blob `locator`. ingot is its consumer |
| `docker/` | — | the one `Dockerfile` every service image builds from |
| `.github/` | — | CI (`ci.yml`), image publishing, releases, and the compatibility workflow |

Every service is a standalone module and is built, tested and packaged as
one. Shared code is reached through `replace` directives in each service's
`go.mod` (`replace github.com/fil-forge/forge/protocol => ../protocol`, and
so on); the root `go.work` exists for editing across modules and is never used
by CI or by the image build. `.github/scripts/check-replaces.sh` verifies that
every in-repo `require` has its `replace`.

## Building and testing

```sh
make build          # every service binary
make test           # every module's unit tests (no Docker)
make vet
make gen-check      # regenerate protocol/attestation codecs, fail on a diff
make check-replaces
make images         # every service's prod image via docker/Dockerfile
make e2e            # smelt's full-stack smoke suite (Docker)
make itest          # the S3 gateway system suite (Docker)
make <module>/<target>   # e.g. make piri/test
```

Inside a module, the standard loop is `GOWORK=off go build ./... && go vet
./... && go test ./...` — `GOWORK=off` is what CI uses, and what proves the
module's `go.mod` is sufficient on its own. piri needs `-tags skiff`.

## Images

```sh
docker build -f docker/Dockerfile --build-arg SERVICE=<dir> --target prod .
```

The build context is the repository root. `MAIN_PKG` selects the main
package (`./cmd` by default; `./cmd/ingot`, `.` for delegator and
piri-signing-service), `BIN` the binary's name inside the image where it
differs from the directory (`registrar`, `signer`, `indexer`), `BUILD_TAGS`
extra build tags. See the Dockerfile header.
