package customer

import (
	"github.com/fil-forge/ucantone/did"
)

type AddArguments struct {
	// DID of the customer account e.g. `did:mailto:agent`
	Customer did.DID `cborgen:"customer" dagjsongen:"customer"`
	// Opaque identifier representing an account in the payment system
	// e.g. Stripe customer ID (stripe:cus_9s6XKzkNRiz8i3)
	ExternalAccount *string `cborgen:"externalAccount,omitempty" dagjsongen:"externalAccount,omitempty"`
	// Unique identifier of the product a.k.a plan.
	Product did.DID `cborgen:"product" dagjsongen:"product"`
	// Misc customer details
	Details map[string]string `cborgen:"details" dagjsongen:"details"`
}
