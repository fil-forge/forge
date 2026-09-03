package access

import (
	"github.com/fil-forge/ucantone/did"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/fil-forge/ucantone/ucan/promise"
	"github.com/ipfs/go-cid"
)

type ClaimOK struct {
	// Delegations is a list of the CIDs of delegations granted for the request.
	Delegations []cid.Cid `cborgen:"delegations" dagjsongen:"delegations"`
}

type RequestArguments struct {
	// DID of the Account authorization is requested from.
	Issuer did.DID `cborgen:"iss" dagjsongen:"iss"`
	// Capabilities agent wishes to be granted.
	Attenuations []CapabilityRequest `cborgen:"att" dagjsongen:"att"`
}

type CapabilityRequest struct {
	Command ucan.Command `cborgen:"cmd" dagjsongen:"cmd"`
}

type RequestOK struct {
	// Request is a link to the access request invocation.
	Request cid.Cid `cborgen:"req" dagjsongen:"req"`
	// Confirm is the task that will confirm the access request.
	Confirm promise.AwaitOK `cborgen:"confirm" dagjsongen:"confirm"`
	// Expiration is the time at which the confirmation will expire.
	Expiration int64 `cborgen:"exp" dagjsongen:"exp"`
}

type ConfirmArguments struct {
	Cause        cid.Cid             `cborgen:"cause" dagjsongen:"cause"`
	Issuer       did.DID             `cborgen:"iss" dagjsongen:"iss"`
	Audience     did.DID             `cborgen:"aud" dagjsongen:"aud"`
	Attenuations []CapabilityRequest `cborgen:"att" dagjsongen:"att"`
}

type DelegateArguments struct {
	// The delegations to store.
	Delegations []cid.Cid `cborgen:"delegations" dagjsongen:"delegations"`
}

type GrantArguments struct {
	// Attenuations are the capabilities the agent wishes to be granted.
	Attenuations []CapabilityRequest `cborgen:"att" dagjsongen:"att"`
	// Cause optionally links to a UCAN invocation that contextualizes the
	// grant request. Leave nil when no cause is needed; otherwise the
	// linked invocation MUST be present in the request container.
	Cause *cid.Cid `cborgen:"cause,omitempty" dagjsongen:"cause,omitempty"`
}
