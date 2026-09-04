//go:build !codegen

package access

import (
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/errors"
	command "github.com/fil-forge/ucantone/ucan/command"
)

// ConfirmOK mirrors ClaimOK — confirming an access request grants the same
// shape of delegations bundle as claiming them.
type ConfirmOK = ClaimOK

// ConfirmMetaKey is the key in metadata in any delegation created by a
// successful access request. The value is a link back to the `/access/confirm`
// invocation.
const ConfirmMetaKey = "accessConfirm"

// Confirm can be invoked by an agent to confirm an access request.
var Confirm = binding.Bind[*ConfirmArguments, *ConfirmOK](command.MustParse("/access/confirm"))

const (
	InvalidAccessConfirmSubjectErrorName = "InvalidAccessConfirmSubject"
	InvalidAccessConfirmIssuerErrorName  = "InvalidAccessConfirmIssuer"
)

var (
	ErrInvalidAccessConfirmSubject = errors.New(InvalidAccessConfirmSubjectErrorName, "the subject of an access confirm invocation must be the service itself")
	ErrInvalidAccessConfirmIssuer  = errors.New(InvalidAccessConfirmIssuerErrorName, "the issuer of an access confirm invocation must be a valid mailto DID")
)
