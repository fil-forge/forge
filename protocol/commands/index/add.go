//go:build !codegen

package index

import (
	"github.com/fil-forge/forge/protocol/commands"
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/errors"
	"github.com/fil-forge/ucantone/ucan/command"
)

type AddOK = commands.Unit

var Add = binding.Bind[*AddArguments, *AddOK](command.MustParse("/index/add"))

const IndexNotFoundErrorName = "IndexNotFound"

var ErrIndexNotFound = errors.New(IndexNotFoundErrorName, "index not found in space")
