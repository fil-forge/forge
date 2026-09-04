module github.com/fil-forge/forge/forgeclient

go 1.27.0

replace github.com/fil-forge/forge/protocol => ../protocol

replace github.com/fil-forge/forge/internal => ../internal

replace github.com/fil-forge/forge/attestation => ../attestation

replace github.com/fil-forge/forge/indexing-service => ../indexing-service

require (
	github.com/fil-forge/forge/attestation v0.0.0
	github.com/fil-forge/forge/indexing-service v0.0.0
	github.com/fil-forge/forge/internal v0.0.0
	github.com/fil-forge/forge/protocol v0.0.0
	github.com/fil-forge/ucantone v0.0.0-20260904190501-7fca40e13941
	github.com/ipfs/go-cid v0.6.2
	github.com/ipfs/go-log/v2 v2.9.2
	github.com/multiformats/go-multihash v0.2.3
	github.com/stretchr/testify v1.12.1
	github.com/whyrusleeping/cbor-gen v0.3.1
	go.uber.org/zap v1.28.0
)

require (
	github.com/alanshaw/dag-json-gen v0.0.9 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.1 // indirect
	github.com/fil-forge/automobile v0.0.1 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/ipni/go-libipni v0.8.2 // indirect
	github.com/klauspost/cpuid/v2 v2.4.0 // indirect
	github.com/libp2p/go-buffer-pool v0.1.0 // indirect
	github.com/libp2p/go-libp2p v0.49.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/minio/sha256-simd v1.0.1 // indirect
	github.com/mr-tron/base58 v1.3.0 // indirect
	github.com/multiformats/go-base32 v0.1.0 // indirect
	github.com/multiformats/go-base36 v0.2.0 // indirect
	github.com/multiformats/go-multiaddr v0.16.1 // indirect
	github.com/multiformats/go-multibase v0.3.0 // indirect
	github.com/multiformats/go-multicodec v0.10.0 // indirect
	github.com/multiformats/go-varint v0.1.0 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.46.0 // indirect
	go.opentelemetry.io/otel/metric v1.46.0 // indirect
	go.opentelemetry.io/otel/trace v1.46.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.55.0 // indirect
	golang.org/x/exp v0.0.0-20260824195058-e88cd73687aa // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/xerrors v0.0.0-20240903120638-7835f813f4da // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	lukechampine.com/blake3 v1.4.1 // indirect
	pitr.ca/jsontokenizer v0.3.2 // indirect
)
