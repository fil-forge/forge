package didmailto

import (
	"context"

	"github.com/fil-forge/forge/attestation"
	"github.com/fil-forge/ucantone/did"
)

func NewResolver(authority did.DID) did.ResolverFunc {
	return func(_ context.Context, d did.DID) (did.Document, error) {
		if err := did.ValidateMethod(d, Method); err != nil {
			return did.Document{}, err
		}

		doc := did.NewDocument(d)
		vm := did.VerificationMethod{
			ID:         doc.Fragment(authority.String()),
			Controller: authority,
			Type:       attestation.Type,
			Material:   did.GenericMap{attestation.AuthorityProp: authority.String()},
		}

		if err := doc.VerificationMethods.Add(vm); err != nil {
			return did.Document{}, err
		}

		if err := doc.CapabilityDelegation.Add(vm); err != nil {
			return did.Document{}, err
		}
		if err := doc.CapabilityInvocation.Add(vm); err != nil {
			return did.Document{}, err
		}

		return doc, nil
	}
}
