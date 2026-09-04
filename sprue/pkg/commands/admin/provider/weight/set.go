//go:build !codegen

package weight

import (
	"github.com/fil-forge/forge/protocol/commands"
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

type SetOK = commands.Unit

var Set = binding.Bind[*SetArguments, *SetOK](command.MustParse("/provider/weight/set"))
