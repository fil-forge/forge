//go:build !codegen

package blob

import (
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

var Accept = binding.Bind[*AcceptArguments, *AcceptOK](command.MustParse("/blob/accept"))
