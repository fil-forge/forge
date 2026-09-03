package bucket

import (
	"github.com/fil-forge/forge/commands/s3"
	"github.com/fil-forge/ucantone/did"
)

// CreateArguments are the arguments to `/s3/bucket/create`.
type CreateArguments struct {
	// Request is the AWS S3 CreateBucket request.
	Request s3.Request `cborgen:"request" dagjsongen:"request"`
}

// DeleteArguments are the arguments to `/s3/bucket/delete`.
type DeleteArguments struct {
	// Request is the AWS S3 DeleteBucket request.
	Request s3.Request `cborgen:"request" dagjsongen:"request"`
}

// ListArguments are the arguments to `/s3/bucket/list`.
type ListArguments struct {
	// Request is the AWS S3 ListBuckets request.
	Request s3.Request `cborgen:"request" dagjsongen:"request"`
}

// InfoArguments are the arguments to `/s3/bucket/info`.
type InfoArguments struct {
	// Name is the global bucket name to look up.
	Name string `cborgen:"name" dagjsongen:"name"`
	// AccessKey is the access key DID the returned delegation chain should
	// terminate at.
	AccessKey did.DID `cborgen:"accessKey" dagjsongen:"accessKey"`
}

// InfoOK is the successful result of `/s3/bucket/info`. Its Permissions and
// Delegations fields are slice-valued maps wrapped in struct types
// ([s3.PermissionSet] etc.) with their own codecs, so this struct is generated
// normally (the generators delegate to a struct field's codec).
type InfoOK struct {
	// ID is the DID of the bucket.
	ID did.DID `cborgen:"id" dagjsongen:"id"`
	// Permissions maps the access key DID to its assigned S3 permissions.
	Permissions s3.PermissionSet `cborgen:"permissions" dagjsongen:"permissions"`
	// Delegations maps the CID of a delegation whose audience is the access key
	// to its proof chain from the bucket.
	Delegations s3.ProofSet `cborgen:"delegations" dagjsongen:"delegations"`
}

// ListOK is the successful result of `/s3/bucket/list`.
type ListOK struct {
	Buckets           []Bucket `cborgen:"buckets" dagjsongen:"buckets"`
	ContinuationToken string   `cborgen:"continuationToken" dagjsongen:"continuationToken"`
	Owner             Owner    `cborgen:"owner" dagjsongen:"owner"`
	Prefix            string   `cborgen:"prefix" dagjsongen:"prefix"`
}

// Bucket describes a single bucket in a `/s3/bucket/list` result.
type Bucket struct {
	ARN          string `cborgen:"arn" dagjsongen:"arn"`
	Region       string `cborgen:"region" dagjsongen:"region"`
	CreationDate string `cborgen:"creationDate" dagjsongen:"creationDate"`
	Name         string `cborgen:"name" dagjsongen:"name"`
}

// Owner is the owner of a set of buckets.
type Owner struct {
	DisplayName string `cborgen:"displayName" dagjsongen:"displayName"`
	ID          string `cborgen:"id" dagjsongen:"id"`
}
