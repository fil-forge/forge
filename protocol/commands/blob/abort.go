//go:build !codegen

package blob

import (
	"github.com/fil-forge/forge/protocol/commands"
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/errors"
	"github.com/fil-forge/ucantone/ucan/command"
)

type AbortOK = commands.Unit

// Abort (/blob/abort) abandons an in-flight upload of a PARKED blob —
// allocated but never accepted, whether or not the bytes ever reached the
// storage node. It is the client-facing abandon verb: an upload ends in
// exactly one of `/blob/accept` (commit) or an abort that the upload service
// translates into `/blob/reject` on the storage node holding the allocation.
//
// Served by the upload service (subject = the space). A parked blob has no
// registration or acceptance to look the storage node up by, so the service
// recovers it from the Cause receipt chain and forwards a `/blob/reject`
// (Cause itself is not forwarded — it is routing metadata, meaningless to
// the node). A missing or unknown Cause fails with MissingCause. Blobs the
// space has accepted are released via `/blob/remove` instead; if the node
// refuses the translated reject with BlobAccepted, the service surfaces that
// named failure in the abort receipt. The abort mutates no upload-service
// state, so a failed abort is safely retryable.
//
// Idempotent: aborting an unknown or already-rejected blob succeeds.
// The receipt carries no payload (Unit).
var Abort = binding.Bind[*AbortArguments, *AbortOK](command.MustParse("/blob/abort"))

// MissingCauseErrorName is the stable receipt-failure name when an abort's
// Cause is missing or does not resolve to a known `/blob/add` task —
// without it the upload service cannot recover which storage node holds the
// parked blob.
const MissingCauseErrorName = "MissingCause"

var ErrMissingCause = errors.New(MissingCauseErrorName, "abort requires the cause of the /blob/add task that parked the blob")
