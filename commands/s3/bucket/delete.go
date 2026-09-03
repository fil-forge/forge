//go:build !codegen

package bucket

import (
	"github.com/fil-forge/forge/commands"
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

// DeleteOK is the (empty) result of a successful `/s3/bucket/delete`.
type DeleteOK = commands.Unit

// Delete is the `/s3/bucket/delete` command. Ingot invokes it on Hilt to
// delete an empty bucket, removing it from Hilt's stores and revoking any
// delegations that grant access to it.
var Delete = binding.Bind[*DeleteArguments, *DeleteOK](command.MustParse("/s3/bucket/delete"))
