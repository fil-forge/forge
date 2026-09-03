//go:build !codegen

package bucket

import (
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

// List is the `/s3/bucket/list` command. Ingot invokes it on Hilt to list the
// buckets belonging to the tenant.
var List = binding.Bind[*ListArguments, *ListOK](command.MustParse("/s3/bucket/list"))
