package attestation

import (
	"context"
	"fmt"
	"strings"

	"github.com/fil-forge/ucantone/did"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/fil-forge/ucantone/validator"
)

var (
	Type          = "AuthorityAttestation"
	AuthorityProp = "authority"
)

// NewVerifierFactory returns a [validator.VerifierFactory] for
// AuthorityAttestation verification methods. Pass it to the validator via
// [validator.WithVerifierFactories]. The provided DID resolver and
// verifierFactories are used to derive verifiers for the authority's own
// verification methods.
func NewVerifierFactory(resolver did.Resolver, verifierFactories map[string]validator.VerifierFactory) validator.VerifierFactory {
	return func(ctx context.Context, mat did.VerificationMaterial) (ucan.Verifier, error) {
		authorityDidStr, ok := mat[AuthorityProp].(string)
		if !ok {
			return nil, fmt.Errorf("AuthorityAttestation verification method missing %s", AuthorityProp)
		}
		authorityDid, err := did.Parse(authorityDidStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse authority DID: %w", err)
		}
		doc, err := resolver.Resolve(ctx, authorityDid)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve authority DID %s: %w", authorityDid, err)
		}

		v, err := newMultiVerifier(ctx, verifierFactories, doc.CapabilityInvocation.All())
		if err != nil {
			return nil, fmt.Errorf("failed to derive multi-verifier: %w", err)
		}
		return AttestedVerifier(ctx, authorityDid, v), nil
	}
}

// multiVerifier is a [ucan.Verifier] that verifies a signature if any of its
// component verifiers verify it. This is useful for cases where a token's
// issuer has multiple verification methods that could have been used to sign
// the token, and the verifier doesn't know which one was used.
type multiVerifier []ucan.Verifier

func (m multiVerifier) Verify(data []byte, sig []byte) bool {
	for _, v := range m {
		if v.Verify(data, sig) {
			return true
		}
	}
	return false
}

func (m multiVerifier) String() string {
	var str strings.Builder
	for _, v := range m {
		str.WriteString(v.String())
	}
	return fmt.Sprintf("multiVerifier{%d verifiers: %s}", len(m), str.String())
}

func newMultiVerifier(ctx context.Context, registry map[string]validator.VerifierFactory, vms []did.VerificationMethod) (ucan.Verifier, error) {
	verifiers := make([]ucan.Verifier, 0, len(vms))
	for _, vm := range vms {
		f, ok := registry[vm.Type]
		if !ok {
			return nil, fmt.Errorf("%w for VM type %q", validator.ErrNoVerifierFactory, vm.Type)
		}
		v, err := f(ctx, vm.Material)
		if err != nil {
			return nil, fmt.Errorf("deriving verifier for VM %s: %w", vm.ID, err)
		}
		verifiers = append(verifiers, v)
	}
	return multiVerifier(verifiers), nil
}
