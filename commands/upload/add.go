//go:build !codegen

package upload

import (
	"github.com/fil-forge/forge/commands"
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

type AddOK = commands.Unit

var Add = binding.Bind[*AddArguments, *AddOK](command.MustParse("/upload/add"))
