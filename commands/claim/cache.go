//go:build !codegen

package claim

import (
	"github.com/fil-forge/forge/commands"
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

type CacheOK = commands.Unit

var Cache = binding.Bind[*CacheArguments, *CacheOK](command.MustParse("/claim/cache"))
