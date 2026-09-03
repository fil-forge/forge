package attest

import "github.com/ipfs/go-cid"

type ProofArguments struct {
	Proof cid.Cid `cborgen:"proof" dagjsongen:"proof"`
}
