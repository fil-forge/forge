package replica

import (
	"github.com/fil-forge/ucantone/ucan/promise"
	"github.com/ipfs/go-cid"

	"github.com/fil-forge/forge/protocol/commands/blob"
)

type AllocateArguments struct {
	// Blob is the blob to be allocated.
	Blob blob.Blob `cborgen:"blob"`
	// Site is a link to a location commitment indicating where the Blob must be
	// fetched from.
	Site cid.Cid `cborgen:"site"`
	// Cause is a link to the `/blob/replicate` task that caused this allocation.
	Cause cid.Cid `cborgen:"cause"`
}

type AllocateOK struct {
	// Site resolves to an additional location for the blob.
	// It is a link to a /blob/replica/transfer task.
	Site promise.AwaitOK `cborgen:"site"`
}

type TransferArguments struct {
	// Blob is the blob to be transferred.
	Blob blob.Blob `cborgen:"blob"`
	// Site is a link to a location commitment indicating where the Blob must be
	// fetched from.
	Site cid.Cid `cborgen:"site"`
	// Cause links to the `/blob/replica/allocate` task that initiated this transfer.
	Cause cid.Cid `cborgen:"cause"`
}

type TransferOK struct {
	// Site links to the location commitment that indicate where the Blob has been
	// transferred to.
	Site cid.Cid `cborgen:"site"`
	// PDP links to the /pdp/accept task that will resolve when aggregation
	// is complete and the piece is accepted.
	PDP promise.AwaitOK `cborgen:"pdp"`
}
