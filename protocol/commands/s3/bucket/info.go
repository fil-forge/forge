//go:build !codegen

package bucket

import (
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

// Info is the `/s3/bucket/info` command. Ingot invokes it on Hilt to look up a
// bucket by global name, returning the bucket DID and the delegation chain
// from the bucket to the given access key.
var Info = binding.Bind[*InfoArguments, *InfoOK](command.MustParse("/s3/bucket/info"))
