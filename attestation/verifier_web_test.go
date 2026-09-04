package attestation_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fil-forge/ucantone/did"
	"github.com/fil-forge/ucantone/did/key"
	"github.com/fil-forge/ucantone/multikey"
	"github.com/fil-forge/ucantone/multikey/ed25519"
	"github.com/fil-forge/ucantone/ucan/command"
	"github.com/fil-forge/ucantone/ucan/delegation"
	"github.com/fil-forge/ucantone/validator"

	"github.com/fil-forge/forge/attestation"
	"github.com/fil-forge/forge/attestation/didmailto"
	"github.com/fil-forge/ucantone/testutil"
)

// TestSigner_WebAuthority exercises an attestation whose authority is a did:web
// service (e.g. did:web:upload), resolved via its DID document — the real
// service flow. The existing TestSigner uses a did:key authority, which the
// default key.Resolver handles, so it never caught that the attestation verifier
// re-resolved the authority with the did:key-only default resolver. With a
// did:web authority that path fails ("signature mismatch"); this test guards the
// fix that verifies with the already-resolved authority verifier.
func TestSigner_WebAuthority(t *testing.T) {
	authority := multikey.NewIssuer(did.MustParse("did:web:example.com"), testutil.Must(ed25519.Generate())(t))
	doc := webDocument(t, authority)

	alice, err := did.Parse("did:mailto:example.com:alice")
	require.NoError(t, err)

	issuer := attestation.Attest(t.Context(), alice, authority)

	del, err := delegation.Delegate(
		issuer,
		testutil.RandomDID(t),
		issuer.DID(),
		command.MustParse("/example/command"),
	)
	require.NoError(t, err)

	encoded, err := delegation.Encode(del)
	require.NoError(t, err)
	decoded, err := delegation.Decode(encoded)
	require.NoError(t, err)

	// Serve the authority's generated did:web document.
	webResolver := did.ResolverFunc(func(_ context.Context, d did.DID) (did.Document, error) {
		if d == authority.DID() {
			return doc, nil
		}
		return did.Document{}, fmt.Errorf("unexpected did %s", d)
	})
	resolver := did.ResolverMap{
		"key":    key.Resolver,
		"web":    webResolver,
		"mailto": didmailto.NewResolver(authority.DID()),
	}
	factories := validator.DefaultFactories()
	factories[attestation.Type] = attestation.NewVerifierFactory(resolver, factories)

	err = validator.ValidateToken(t.Context(), decoded,
		validator.WithDIDResolver(resolver),
		validator.WithVerifierFactories(factories),
	)
	require.NoError(t, err)
}

// webDocument builds the DID document a did:web service would publish for
// itself: one verification method derived from the issuer's key, listed under
// authentication and assertionMethod.
func webDocument(t *testing.T, iss multikey.Issuer) did.Document {
	t.Helper()
	doc := did.NewDocument(iss.DID())
	verifier, ok := iss.Verifier().(multikey.Verifier)
	require.True(t, ok, "issuer must have a multikey verifier")
	vm := multikey.DeriveVerificationMethod(doc.Fragment("key-0"), verifier)
	vm.Controller = doc.ID
	require.NoError(t, doc.VerificationMethods.Add(vm))
	require.NoError(t, doc.Authentication.Add(vm))
	require.NoError(t, doc.AssertionMethod.Add(vm))
	return doc
}
