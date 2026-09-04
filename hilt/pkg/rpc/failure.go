package rpc

import (
	"errors"
	s3bkt "github.com/fil-forge/forge/protocol/commands/s3/bucket"
	s3req "github.com/fil-forge/forge/protocol/commands/s3/request"
)

// failer is the subset of *binding.Response[OK] used to record a receipt failure.
type failer interface{ SetFailure(error) error }

// authFailure records a known auth-service rejection as the invocation's receipt
// failure, so its stable Name() reaches the caller (Ingot maps it to a canonical S3
// error). SetFailure unwraps to find the Name, so the service's %w-wrapped error is
// passed as-is (its full message is preserved). Unknown/internal errors are returned
// unchanged; the dispatcher then reports them as a "HandlerExecutionError".
func authFailure(res failer, err error) error {
	switch {
	case errors.Is(err, s3req.ErrMalformedSignature),
		errors.Is(err, s3req.ErrInvalidAccessKeyID),
		errors.Is(err, s3req.ErrUnknownAccessKey),
		errors.Is(err, s3req.ErrSignatureMismatch),
		errors.Is(err, s3req.ErrSignatureExpired),
		errors.Is(err, s3req.ErrAccessKeyExpired),
		errors.Is(err, s3req.ErrTenantDisabled),
		errors.Is(err, s3req.ErrIssuerForbidden),
		errors.Is(err, s3req.ErrRegionNotServed),
		errors.Is(err, s3req.ErrUnsupportedOperation),
		errors.Is(err, s3req.ErrOperationNotPermitted),
		errors.Is(err, s3req.ErrUnknownBucket),
		errors.Is(err, s3req.ErrBucketNotPermitted):
		return res.SetFailure(err)
	default:
		return err
	}
}

// adminFailure records a known admin-command rejection as the receipt failure so
// its stable Name() reaches the caller; other errors are returned unchanged (→
// "HandlerExecutionError").
func adminFailure(res failer, err error) error {
	switch {
	case errors.Is(err, ErrUnauthorized),
		errors.Is(err, ErrProviderExists):
		return res.SetFailure(err)
	default:
		return err
	}
}

// bucketFailure records a known bucket-service rejection, falling through to
// authFailure for the auth rejections the bucket service propagates from Authorize.
func bucketFailure(res failer, err error) error {
	switch {
	case errors.Is(err, s3bkt.ErrOperationMismatch),
		errors.Is(err, s3bkt.ErrBucketExists),
		errors.Is(err, s3bkt.ErrBucketAlreadyOwned),
		errors.Is(err, s3bkt.ErrBucketNotEmpty),
		errors.Is(err, s3bkt.ErrUnknownBucket),
		errors.Is(err, s3bkt.ErrUnknownAccessKey),
		errors.Is(err, s3bkt.ErrInvalidArgument):
		return res.SetFailure(err)
	default:
		return authFailure(res, err)
	}
}
