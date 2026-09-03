//go:build !codegen

package assert

import (
	"github.com/fil-forge/forge/commands"
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

type LocationOK = commands.Unit

var Location = binding.Bind[*LocationArguments, *LocationOK](command.MustParse("/assert/location"))
