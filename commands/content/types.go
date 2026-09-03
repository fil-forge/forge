package content

import "github.com/multiformats/go-multihash"

type Blob struct {
	Digest multihash.Multihash `cborgen:"digest" dagjsongen:"digest"`
}

type Range struct {
	Start uint64 `cborgen:"start" dagjsongen:"start"`
	End   uint64 `cborgen:"end" dagjsongen:"end"`
}

type RetrieveArguments struct {
	Blob  Blob  `cborgen:"blob" dagjsongen:"blob"`
	Range Range `cborgen:"range" dagjsongen:"range"`
}
