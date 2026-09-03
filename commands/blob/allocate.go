//go:build !codegen

package blob

import (
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

const MaxBlobSize = 268_435_456

var Allocate = binding.Bind[*AllocateArguments, *AllocateOK](command.MustParse("/blob/allocate"))
