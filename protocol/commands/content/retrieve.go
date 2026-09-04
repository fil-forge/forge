//go:build !codegen

package content

import (
	"github.com/fil-forge/forge/protocol/commands"
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

type RetrieveOK = commands.Unit

var Retrieve = binding.Bind[*RetrieveArguments, *RetrieveOK](command.MustParse("/content/retrieve"))
