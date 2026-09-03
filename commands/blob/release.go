//go:build !codegen

package blob

import (
	"github.com/fil-forge/forge/commands"
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/errors"
	"github.com/fil-forge/ucantone/ucan/command"
)

type ReleaseOK = commands.Unit

// Release (/blob/release) is the claim-release inverse of `/blob/allocate`:
// it drops one space's reference to a blob on a storage node. It is the
// upload service's translation of a client `/blob/remove`.
//
// Served by storage nodes (subject = the provider DID, invoked by the upload
// service under its registration delegation; the space travels in the
// arguments, matching Allocate/Accept). The node drops the space's
// allocation, acceptance and location claim. Bytes are physically deleted
// only when no space claims the digest anymore — and an accepted blob's
// bytes may additionally be retained until its PDP aggregate root is fully
// retired on-chain. Physical deletion is always asynchronous: the removal
// machinery re-verifies zero claims before every destructive step.
//
// The release carries Cause, a link to the originating `/blob/remove` task,
// and the remove invocation itself MUST ride in the request container so the
// node can verify the release is legitimate rather than trusting the upload
// service: the node rejects releases whose cause invocation is absent or not
// a `/blob/remove` task (UnknownCause), or whose subject/digest do not match
// the release's Space/Digest (InvalidCause).
//
// Idempotent: releasing an unknown or already-released blob succeeds. The
// receipt carries no payload (Unit).
var Release = binding.Bind[*ReleaseArguments, *ReleaseOK](command.MustParse("/blob/release"))

// UnknownCauseErrorName is the stable receipt-failure name when a release's
// Cause is undefined, the linked invocation is not present in the request
// container, or the linked invocation is not a `/blob/remove` task.
const UnknownCauseErrorName = "UnknownCause"

var ErrUnknownCause = errors.New(UnknownCauseErrorName, "unknown cause invocation")

// InvalidCauseErrorName is the stable receipt-failure name when the linked
// `/blob/remove` task's subject does not equal Space or its digest does not
// equal Digest.
const InvalidCauseErrorName = "InvalidCause"

var ErrInvalidCause = errors.New(InvalidCauseErrorName, "cause invocation does not match release arguments")
