package blob

import (
	"github.com/fil-forge/forge/protocol/commands"
	"github.com/fil-forge/ucantone/did"
	"github.com/fil-forge/ucantone/ucan/promise"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
)

type Blob struct {
	Digest multihash.Multihash `cborgen:"digest" dagjsongen:"digest"`
	Size   uint64              `cborgen:"size" dagjsongen:"size"`
}

type AddArguments struct {
	Blob Blob `cborgen:"blob" dagjsongen:"blob"`
}

type AddOK struct {
	// Site is a promise of the `/blob/accept` task result, which contains a
	// location claim for the blob, describing where it can be retrieved from.
	Site promise.AwaitOK `cborgen:"site" dagjsongen:"site"`
}

type AcceptArguments struct {
	Space did.DID         `cborgen:"space" dagjsongen:"space"`
	Blob  Blob            `cborgen:"blob" dagjsongen:"blob"`
	Put   promise.AwaitOK `cborgen:"_put" dagjsongen:"_put"`
}

type AcceptOK struct {
	Site cid.Cid `cborgen:"site" dagjsongen:"site"`
	// PDP is a promise of the `/pdp/accept` task result, which completes when
	// the piece has been aggregated and root added to the node's PDP dataset.
	PDP promise.AwaitOK `cborgen:"pdp" dagjsongen:"pdp"`
}

type AllocateArguments struct {
	Space did.DID `cborgen:"space" dagjsongen:"space"`
	Blob  Blob    `cborgen:"blob" dagjsongen:"blob"`
	Cause cid.Cid `cborgen:"cause" dagjsongen:"cause"`
}

type AllocateOK struct {
	Size    uint64       `cborgen:"size" dagjsongen:"size"`
	Address *BlobAddress `cborgen:"address,omitempty" dagjsongen:"address,omitempty"`
}

type BlobAddress struct {
	URL     commands.CborURL  `cborgen:"url" dagjsongen:"url"`
	Headers map[string]string `cborgen:"headers" dagjsongen:"headers"`
	Expires int64             `cborgen:"expires" dagjsongen:"expires"`
}

type ListArguments struct {
	Cursor *string `cborgen:"cursor,omitempty" dagjsongen:"cursor,omitempty"`
	Size   *uint64 `cborgen:"size,omitempty" dagjsongen:"size,omitempty"`
}

type ListOK struct {
	Cursor  *string        `cborgen:"cursor,omitempty" dagjsongen:"cursor,omitempty"`
	Size    uint64         `cborgen:"size" dagjsongen:"size"`
	Results []ListBlobItem `cborgen:"results" dagjsongen:"results"`
}

type ListBlobItem struct {
	Blob       Blob  `cborgen:"blob" dagjsongen:"blob"`
	InsertedAt int64 `cborgen:"insertedAt" dagjsongen:"insertedAt"`
}

// RemoveArguments releases the invoking space's claim on the blob
// identified by Digest. The space is the invocation subject — it is not
// repeated in the arguments (compare ReleaseArguments, the provider-rooted
// leg, where the subject is the provider and the space must travel
// explicitly).
type RemoveArguments struct {
	Digest multihash.Multihash `cborgen:"digest" dagjsongen:"digest"`
}

// ReleaseArguments drops Space's claim on the blob identified by Digest on a
// storage node. Space is explicit (matching Allocate/Accept) because the
// invocation subject is the provider, and storage nodes key allocations and
// acceptances by (digest, space): release drops one space's claim, and the
// node performs physical deletion only once no space claims the digest at
// all. Cause proves the release originates from the space: it links the
// `/blob/remove` task the release translates.
type ReleaseArguments struct {
	Space  did.DID             `cborgen:"space" dagjsongen:"space"`
	Digest multihash.Multihash `cborgen:"digest" dagjsongen:"digest"`
	// Cause is the link to the `/blob/remove` task this release translates.
	// The linked invocation MUST be present in the request container; the
	// node verifies its subject equals Space and its digest equals Digest
	// before dropping the claim.
	Cause cid.Cid `cborgen:"cause" dagjsongen:"cause"`
}

// AbortArguments abandons the invoking space's in-flight upload of the
// parked (never-accepted) blob identified by Digest. The space is the
// invocation subject. Cause is the `/blob/add` task link: the upload
// service uses it to recover which storage node holds the parked blob — a
// parked blob has no registration or acceptance to look the node up by.
type AbortArguments struct {
	Digest multihash.Multihash `cborgen:"digest" dagjsongen:"digest"`
	Cause  cid.Cid             `cborgen:"cause" dagjsongen:"cause"`
}

// RejectArguments drops Space's allocation for the parked (never-accepted)
// blob identified by Digest on the storage node; the node deletes any
// received bytes once no space holds an allocation or acceptance for the
// digest.
type RejectArguments struct {
	Space  did.DID             `cborgen:"space" dagjsongen:"space"`
	Digest multihash.Multihash `cborgen:"digest" dagjsongen:"digest"`
}

type ReplicateArguments struct {
	// Blob is the blob that must be replicated.
	Blob Blob `cborgen:"blob" dagjsongen:"blob"`
	// Replicas is the number of replicas to ensure.
	// e.g. Replicas: 3 will ensure 3 copies of the data exist in a network in total.
	Replicas uint64 `cborgen:"replicas" dagjsongen:"replicas"`
	// Site is a link to a location commitment indicating where the Blob must be
	// fetched from.
	Site cid.Cid `cborgen:"site" dagjsongen:"site"`
}

type ReplicateOK struct {
	// Site resolves to additional locations for the blob. They are links to
	// `/blob/replica/transfer` tasks.
	Site []promise.AwaitOK `cborgen:"site" dagjsongen:"site"`
}

// RetrieveBlob identifies a blob solely by its content multihash. Used by the
// service-level `/blob/retrieve` capability where the caller is fetching data
// by hash without prior knowledge of the byte size.
type RetrieveBlob struct {
	Digest multihash.Multihash `cborgen:"digest" dagjsongen:"digest"`
}

// RetrieveArguments is the argument shape of the `/blob/retrieve` capability —
// a service-level (not space-scoped) retrieval handle. Compare to
// `content.RetrieveArguments` which is space-scoped and carries a byte Range;
// the `/blob/retrieve` flow is consumed by service principals (e.g. the
// indexer) fetching content claims that aren't bound to any space.
type RetrieveArguments struct {
	Blob RetrieveBlob `cborgen:"blob" dagjsongen:"blob"`
}

// RetrieveOK is the success return for `/blob/retrieve`. The blob bytes
// themselves are streamed back through the response container's body (the
// retrieval package's HTTPHeader transport); the typed OK record is empty.
type RetrieveOK struct{}
