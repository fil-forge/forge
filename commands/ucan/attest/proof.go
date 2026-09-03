//go:build !codegen

package attest

import (
	"github.com/fil-forge/forge/commands"
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

type ProofOK = commands.Unit

// Issued by a trusted authority (usually the one handling invocation) that
// attests a specific UCAN delegation has been considered authentic.
var Proof = binding.Bind[*ProofArguments, *ProofOK](command.MustParse("/ucan/attest/proof"))
