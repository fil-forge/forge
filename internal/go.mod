module github.com/fil-forge/forge/internal

go 1.27.0

// In-repo sibling. A replace here is fine: nothing outside this repository can
// import an internal package, so no consumer ever sees this go.mod.
replace github.com/fil-forge/forge/commands => ../commands

require (
	github.com/fil-forge/forge/commands v0.0.0-00010101000000-000000000000
	github.com/fil-forge/libforge v0.0.0-20260828121550-2585ed1e5e50
	github.com/fil-forge/ucantone v0.0.0-20260828153820-8d7eb73066ce
	github.com/ipfs/go-cid v0.6.2
	github.com/multiformats/go-multibase v0.3.0
	github.com/multiformats/go-multihash v0.2.3
	github.com/stretchr/testify v1.12.1
	go.uber.org/zap v1.28.0
)

require (
	github.com/alanshaw/dag-json-gen v0.0.9 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/klauspost/cpuid/v2 v2.4.0 // indirect
	github.com/minio/sha256-simd v1.0.1 // indirect
	github.com/mr-tron/base58 v1.3.0 // indirect
	github.com/multiformats/go-base32 v0.1.0 // indirect
	github.com/multiformats/go-base36 v0.2.0 // indirect
	github.com/multiformats/go-multicodec v0.10.0 // indirect
	github.com/multiformats/go-varint v0.1.0 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/whyrusleeping/cbor-gen v0.3.2-0.20250409092040-76796969edea // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.55.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/xerrors v0.0.0-20240903120638-7835f813f4da // indirect
	lukechampine.com/blake3 v1.4.1 // indirect
	pitr.ca/jsontokenizer v0.3.2 // indirect
)
