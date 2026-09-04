package pdp

import (
	"github.com/filecoin-project/go-data-segment/merkletree"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
)

type AcceptArguments struct {
	Blob multihash.Multihash `cborgen:"blob" dagjsongen:"blob"`
}

type AcceptOK struct {
	Aggregate      cid.Cid              `cborgen:"aggregate" dagjsongen:"aggregate"`
	InclusionProof merkletree.ProofData `cborgen:"inclusionProof" dagjsongen:"inclusionProof"`
	Piece          cid.Cid              `cborgen:"piece" dagjsongen:"piece"`
}

type InfoArguments struct {
	Blob multihash.Multihash `cborgen:"blob" dagjsongen:"blob"`
}

type InfoAcceptedAggregate struct {
	Aggregate      cid.Cid              `cborgen:"aggregate" dagjsongen:"aggregate"`
	InclusionProof merkletree.ProofData `cborgen:"inclusionProof" dagjsongen:"inclusionProof"`
}

type InfoOK struct {
	Piece      cid.Cid                 `cborgen:"piece" dagjsongen:"piece"`
	Aggregates []InfoAcceptedAggregate `cborgen:"aggregates" dagjsongen:"aggregates"`
}
