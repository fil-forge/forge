//go:build !codegen

package pdp

import (
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

var Accept = binding.Bind[*AcceptArguments, *AcceptOK](command.MustParse("/pdp/accept"))
