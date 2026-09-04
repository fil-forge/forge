//go:build !codegen

package ucan

import (
	"github.com/fil-forge/forge/protocol/commands"
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/errors"
	"github.com/fil-forge/ucantone/ucan/command"
)

type ConcludeOK = commands.Unit

var Conclude = binding.Bind[*ConcludeArguments, *ConcludeOK](command.MustParse("/ucan/conclude"))

const ConclusionReceiptNotFoundErrorName = "ConclusionReceiptNotFound"

var ErrConclusionReceiptNotFound = errors.New(ConclusionReceiptNotFoundErrorName, "conclusion receipt not found")
