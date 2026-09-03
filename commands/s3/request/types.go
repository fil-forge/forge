package request

import (
	"github.com/fil-forge/forge/commands/s3"
	"github.com/fil-forge/ucantone/did"
)

// AuthorizeArguments are the arguments to the `/s3/request/authorize` command.
type AuthorizeArguments struct {
	// Request is the AWS S3 API request to authorize.
	Request s3.Request `cborgen:"request" dagjsongen:"request"`
}

// AuthorizeOK is the successful result of `/s3/request/authorize`. It carries
// the resolved bucket DID, the tenant DID, the S3 permission set for the access
// key, the derived signing key(s) and the (24-hour TTL) delegations
// re-delegated to the invocation issuer.
//
// Its Permissions, Keys and Delegations fields are slice-valued maps that
// cbor-gen / dag-json-gen cannot generate inline, but they are wrapped in
// struct types ([s3.PermissionSet] etc.) with their own codecs, so this struct
// is generated normally (the generators delegate to a struct field's codec).
type AuthorizeOK struct {
	// Bucket is the DID of the bucket addressed by the request. Note: not all
	// requests are bucket-scoped, so this field may be nil. e.g. CreateBucket,
	// ListAllMyBuckets, etc.
	Bucket *did.DID `cborgen:"bucket,omitempty" dagjsongen:"bucket,omitempty"`
	// Tenant is the DID of the tenant the access key belongs to (the tenant's
	// did:plc). The gateway resolves its DID document to obtain the tenant's
	// wrap key — the FEE tenant recipient every stored object is encrypted to.
	Tenant did.DID `cborgen:"tenant" dagjsongen:"tenant"`
	// Permissions maps the access key DID to its assigned S3 permissions.
	Permissions s3.PermissionSet `cborgen:"permissions" dagjsongen:"permissions"`
	// Keys maps the access key DID to its derived signing key(s).
	Keys s3.KeySet `cborgen:"keys" dagjsongen:"keys"`
	// Delegations maps the CID of a delegation whose audience is the invocation
	// issuer to its proof chain.
	Delegations s3.ProofSet `cborgen:"delegations" dagjsongen:"delegations"`
}
