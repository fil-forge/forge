//go:build !codegen

package space

import (
	"github.com/fil-forge/forge/commands"
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

type InfoArguments = commands.Unit

var Info = binding.Bind[*InfoArguments, *InfoOK](command.MustParse("/space/info"))

const UnknownSpaceErrorName = "UnknownSpace"
