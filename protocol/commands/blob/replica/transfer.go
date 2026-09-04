//go:build !codegen

package replica

import (
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

var Transfer = binding.Bind[*TransferArguments, *TransferOK](command.MustParse("/blob/replica/transfer"))
