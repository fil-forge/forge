//go:build !codegen

package ucan

import (
	"github.com/fil-forge/forge/protocol/commands"
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

type RevokeOK = commands.Unit

var Revoke = binding.Bind[*RevokeArguments, *RevokeOK](command.MustParse("/ucan/revoke"))
