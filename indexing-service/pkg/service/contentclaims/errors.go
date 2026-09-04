package contentclaims

import "github.com/fil-forge/ucantone/errors"

var ErrMissingClaim = errors.New("MissingClaim", "Claim data was not found in the invocation payload.")
