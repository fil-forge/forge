//go:build !codegen

package blob

import (
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

var Add = binding.Bind[*AddArguments, *AddOK](command.MustParse("/blob/add"))
