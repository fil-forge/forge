//go:build !codegen

package request

import (
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

// Authorize is the `/s3/request/authorize` command. Ingot invokes it on Hilt
// (issuer = Ingot, audience = subject = Hilt) to authorize an AWS S3 API
// request: Hilt verifies the SigV4 signature, looks up the access key's
// delegations, derives a signing key and re-delegates capabilities to the
// invocation issuer.
var Authorize = binding.Bind[*AuthorizeArguments, *AuthorizeOK](command.MustParse("/s3/request/authorize"))
