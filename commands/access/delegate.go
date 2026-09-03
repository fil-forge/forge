//go:build !codegen

package access

import (
	"github.com/fil-forge/forge/commands"
	"github.com/fil-forge/ucantone/binding"
	command "github.com/fil-forge/ucantone/ucan/command"
)

type DelegateOK = commands.Unit

// Delegate can be invoked by an agent to delegate a set of capabilities that
// may be subsequently claimed by another agent.
var Delegate = binding.Bind[*DelegateArguments, *DelegateOK](command.MustParse("/access/delegate"))

const (
	DelegationNotFoundErrorName  = "DelegationNotFound"
	InsufficientStorageErrorName = "InsufficientStorage"
)
