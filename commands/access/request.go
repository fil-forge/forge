//go:build !codegen

package access

import (
	"github.com/fil-forge/ucantone/binding"
	command "github.com/fil-forge/ucantone/ucan/command"
)

// RequestFactKey is the key in metadata in any delegation created by a
// successful access request. The value is a link back to the `/access/request`
// invocation.
const RequestMetaKey = "accessRequest"

// Request can be invoked by an agent to request set of capabilities from the
// account.
var Request = binding.Bind[*RequestArguments, *RequestOK](command.MustParse("/access/request"))

const (
	InvalidAuthorizationAccountErrorName  = "InvalidAuthorizationAccount"
	InvalidAuthorizationAudienceErrorName = "InvalidAuthorizationAudience"
)
