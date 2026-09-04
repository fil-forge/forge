package assert

import (
	"github.com/fil-forge/forge/protocol/commands"
	"github.com/fil-forge/ucantone/did"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
)

type IndexArguments struct {
	Index cid.Cid `cborgen:"index" dagjsongen:"index"`
}

type IndexMetadata struct {
	// RetrievalAuth is a list of CIDs of delegations (in order required for
	// invocation) that can be used as proofs to issue a `/content/retrieve`
	// invocation to retrieve an index blob.
	RetrievalAuth []cid.Cid `cborgen:"retrievalAuth" dagjsongen:"retrievalAuth"`
}

type LocationArguments struct {
	Space    did.DID             `cborgen:"space" dagjsongen:"space"`
	Content  multihash.Multihash `cborgen:"content" dagjsongen:"content"`
	Location []commands.CborURL  `cborgen:"location" dagjsongen:"location"`
	Range    *Range              `cborgen:"range,omitempty" dagjsongen:"range,omitempty"`
}

type Range struct {
	Start uint64  `cborgen:"start" dagjsongen:"start"`
	End   *uint64 `cborgen:"end,omitempty" dagjsongen:"end,omitempty"`
}

type EqualsArguments struct {
	Content multihash.Multihash `cborgen:"content" dagjsongen:"content"`
	Equals  cid.Cid             `cborgen:"equals" dagjsongen:"equals"`
}
