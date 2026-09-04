//go:build !codegen

package pdp

import (
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

const InfoCommand = "/pdp/info"

var Info = binding.Bind[*InfoArguments, *InfoOK](command.MustParse(InfoCommand))
