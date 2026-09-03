//go:build !codegen

package debug

import (
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

type EchoOK = EchoArguments

var Echo = binding.Bind[*EchoArguments, *EchoOK](command.MustParse("/debug/echo"))
