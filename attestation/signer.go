package attestation

import (
	"context"
	"fmt"

	"github.com/fil-forge/forge/attestation/attest"
	"github.com/fil-forge/ucantone/did"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/fil-forge/ucantone/ucan/invocation"
	"github.com/fil-forge/ucantone/varsig"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

func Attest(ctx context.Context, subject did.DID, authority ucan.Issuer) Issuer {
	return Issuer{
		ctx:       ctx,
		did:       subject,
		authority: authority,
	}
}

type Issuer struct {
	ctx       context.Context
	did       did.DID
	authority ucan.Issuer
}

var _ ucan.Issuer = Issuer{}

func (s Issuer) DID() did.DID {
	return s.did
}

func (s Issuer) String() string {
	return fmt.Sprintf("%s (attested by %s)", s.did, s.authority.DID())
}

func (s Issuer) Sign(data []byte) []byte {
	msgDigest, err := mh.Sum(data, mh.SHA2_256, -1)
	if err != nil {
		panic(fmt.Sprintf("failed to compute message digest: %v", err))
	}

	// The attestation is an assertion of fact: it is inherently idempotent
	// (no nonce) and cannot expire (the attested delegation's own exp bounds
	// the authorization). See the attested authority spec, signature
	// invocation fields.
	inv, err := attest.Proof.Invoke(
		s.authority,
		s.authority.DID(),
		&attest.ProofArguments{Proof: cid.NewCidV1(cid.Raw, msgDigest)},
		invocation.WithNoNonce(),
		invocation.WithNoExpiration(),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create invocation: %v", err))
	}
	return inv.Bytes()
}

func (s Issuer) SignatureAlgorithm() varsig.Algorithm {
	return Algorithm{}
}

func (s Issuer) Verifier() ucan.Verifier {
	return AttestedVerifier(s.ctx, s.authority.DID(), s.authority.Verifier())
}
