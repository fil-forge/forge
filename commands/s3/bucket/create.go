//go:build !codegen

package bucket

import (
	"github.com/fil-forge/forge/commands/s3/request"
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

// Create is the `/s3/bucket/create` command. Ingot invokes it on Hilt to
// create a bucket and provision it with Sprue. It returns the same structure
// as `/s3/request/authorize` ([request.AuthorizeOK]): the new bucket DID and
// any delegations that now have access to it.
var Create = binding.Bind[*CreateArguments, *request.AuthorizeOK](command.MustParse("/s3/bucket/create"))
