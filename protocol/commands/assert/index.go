//go:build !codegen

package assert

import (
	"github.com/fil-forge/forge/protocol/commands"
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

type IndexOK = commands.Unit

// Index claims that a content graph can be found in blob(s) that are identified
// and indexed in the given index CID.
var Index = binding.Bind[*IndexArguments, *IndexOK](command.MustParse("/assert/index"))
