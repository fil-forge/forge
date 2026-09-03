package egress

import (
	"github.com/fil-forge/forge/commands"
	"github.com/ipfs/go-cid"
)

// TrackArguments is the argument shape of `/space/egress/track`. A storage
// node invokes this capability to ask the egress tracking service to record
// the egress accounted for in a batch of `/content/retrieve` receipts.
// Receipts is the CID of the receipts batch (root of the dag-cbor archive
// the storage node has staged); Endpoint is the URL the tracking service
// should fetch that archive from.
type TrackArguments struct {
	Receipts cid.Cid          `cborgen:"receipts" dagjsongen:"receipts"`
	Endpoint commands.CborURL `cborgen:"endpoint" dagjsongen:"endpoint"`
}

// TrackOK is the success return for `/space/egress/track`. The tracking
// service's consolidation response is delivered out-of-band as a forked
// sub-invocation (a `/space/egress/consolidate`) attached to the receipt's
// effects; the typed OK record itself is empty.
type TrackOK struct{}
