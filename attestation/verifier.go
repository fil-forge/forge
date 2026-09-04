package attestation

import (
	"bytes"
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"

	"github.com/fil-forge/forge/attestation/attest"
	"github.com/fil-forge/ucantone/did"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/fil-forge/ucantone/ucan/invocation"
	"github.com/fil-forge/ucantone/ucan/token"
)

// Verifier is a ucan.Verifier for a DID whose signing is attested by an
// authority verifier.
type Verifier struct {
	ctx               context.Context
	authorityID       did.DID
	authorityVerifier ucan.Verifier
}

var _ ucan.Verifier = Verifier{}

func (v Verifier) String() string {
	return fmt.Sprintf("Attested Verifier{authority=%s}", v.authorityID)
}

func (v Verifier) Verify(msg []byte, sig []byte) bool {
	inv, err := invocation.Decode(sig)
	if err != nil {
		return false
	}

	var args attest.ProofArguments
	err = args.UnmarshalCBOR(bytes.NewReader(inv.ArgumentsBytes()))
	if err != nil {
		return false
	}

	msgDigest, err := mh.Sum(msg, mh.SHA2_256, -1)
	if err != nil {
		return false
	}

	if args.Proof != cid.NewCidV1(cid.Raw, msgDigest) {
		return false
	}

	if inv.Subject() != v.authorityID {
		return false
	}

	// Verify the attestation invocation was actually signed by the authority,
	// using the already-resolved authority verifier directly. Previously this
	// re-validated via validator.ValidateInvocation with no options, which
	// defaults to a did:key-only resolver and therefore fails to resolve a
	// did:web authority — the cause of "signature mismatch" for did:web services
	// (e.g. did:web:upload) while did:key authorities (and unit tests) passed.
	if !token.VerifySignature(inv, v.authorityVerifier) {
		return false
	}

	return true
}

func AttestedVerifier(ctx context.Context, authorityID did.DID, authority ucan.Verifier) ucan.Verifier {
	return Verifier{
		ctx:               ctx,
		authorityID:       authorityID,
		authorityVerifier: authority,
	}
}
