//go:build !codegen

package replica

import (
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

var Allocate = binding.Bind[*AllocateArguments, *AllocateOK](command.MustParse("/blob/replica/allocate"))
