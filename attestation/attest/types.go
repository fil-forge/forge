package attest

import "github.com/ipfs/go-cid"

type ProofArguments struct {
	Proof cid.Cid `cborgen:"proof" dagjsongen:"proof"`
}

// Unit is the empty wire type the attestation receipt carries: it encodes as
// an empty CBOR map / dag-json object.
type Unit struct{}
