//go:build !codegen

package blob

import (
	"github.com/fil-forge/forge/protocol/commands"
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/errors"
	"github.com/fil-forge/ucantone/ucan/command"
)

type RejectOK = commands.Unit

// Reject (/blob/reject) is the "don't accept" inverse of `/blob/accept`: it
// retires a PARKED blob — allocated, never accepted, whether or not its
// bytes were ever received. A blob's lifecycle on a storage node ends in
// exactly one of `/blob/accept` (commit: aggregation, location claim,
// registration) or `/blob/reject` (drop: allocation released, bytes deleted).
//
// Served by storage nodes (subject = the provider DID, invoked by the
// upload service under its registration delegation, typically translating a
// client `/blob/abort`). The node drops the space's allocation and deletes
// any received bytes once no space holds an allocation or acceptance for
// the digest.
//
// A blob that THE INVOKING SPACE has accepted is refused with BlobAccepted —
// a space's accepted blobs are released via `/blob/remove`, never rejected.
// The guard is scoped to the invoking space, not the digest: another space's
// acceptance of the same bytes must not block the reject — the node simply
// drops this space's allocation and retains the bytes for the space that
// still claims them.
//
// Idempotent: rejecting an unknown or already-rejected blob succeeds.
// The receipt carries no payload (Unit).
var Reject = binding.Bind[*RejectArguments, *RejectOK](command.MustParse("/blob/reject"))

// BlobAcceptedErrorName is the stable receipt-failure name when reject is
// invoked for a blob the invoking space has accepted — accepted blobs are
// released via `/blob/remove`, never rejected.
const BlobAcceptedErrorName = "BlobAccepted"

var ErrBlobAccepted = errors.New(BlobAcceptedErrorName, "blob has been accepted by the space; release the claim via /blob/remove")
