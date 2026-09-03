//go:build !codegen

package access

import (
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/errors"
	"github.com/fil-forge/ucantone/ucan/command"
)

const GrantCommand = "/access/grant"

// GrantOK mirrors ClaimOK / ConfirmOK: a successful grant resolves into a
// bundle of delegation CIDs. The actual delegation envelopes ride in the
// receipt response container as metadata.
type GrantOK = ClaimOK

// Grant can be invoked by an agent to request that a set of capabilities be
// granted directly. Unlike Request -> Confirm, Grant is one-shot: the
// executor decides immediately whether to issue the delegation.
var Grant = binding.Bind[*GrantArguments, *GrantOK](command.MustParse(GrantCommand))

const (
	UnknownAbilityErrorName    = "UnknownAbility"
	MissingCapabilityErrorName = "MissingCapability"
	UnknownCauseErrorName      = "UnknownCause"
	MissingCauseErrorName      = "MissingCause"
	InvalidCauseErrorName      = "InvalidCause"
	UnauthorizedCauseErrorName = "UnauthorizedCause"
)

var (
	ErrMissingCapability = errors.New(MissingCapabilityErrorName, "grant requires one or more capabilities")
	ErrMissingCause      = errors.New(MissingCauseErrorName, "grant requires a supporting contextual invocation")
	ErrUnknownCause      = errors.New(UnknownCauseErrorName, "unknown cause invocation")
)
