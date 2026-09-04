package identity

import (
	"fmt"
	"os"

	"github.com/fil-forge/ucantone/did"
	"github.com/fil-forge/ucantone/multikey"
	"github.com/fil-forge/ucantone/multikey/ed25519"
)

// Identity holds a service's cryptographic identity. It's intended to be held
// by the service itself. This is the source of its DID document, can then be
// published (eg, to the web). Other services should use normal DID resolution
// to find the document.
type Identity struct {
	multikey.Issuer
}

// New creates a new identity. If privateKeyBase64 is empty, generates a new
// key. If serviceDID is empty, uses the key DID derived from the key.
func New(privateKeyBase64 string, serviceDID string) (Identity, error) {
	var signer multikey.Signer
	var issuer multikey.Issuer
	var err error

	if privateKeyBase64 == "" {
		// Generate ephemeral identity
		signer, err = ed25519.Generate()
		if err != nil {
			return Identity{}, fmt.Errorf("failed to generate signer: %w", err)
		}
	} else {
		// Decode provided key
		signer, err = ed25519.Parse(privateKeyBase64)
		if err != nil {
			return Identity{}, fmt.Errorf("failed to create signer from key: %w", err)
		}
	}

	if serviceDID == "" {
		issuer = multikey.KeyIssuer(signer)
	} else {
		d, err := did.Parse(serviceDID)
		if err != nil {
			return Identity{}, fmt.Errorf("failed to parse service DID %q: %w", serviceDID, err)
		}
		issuer = multikey.NewIssuer(d, signer)
	}

	return Identity{Issuer: issuer}, nil
}

// DIDDocument returns the identity's DID document. This should be available for
// other services performing did:web resolution. This enables other services to
// verify signatures from this service.
func (i Identity) DIDDocument() (did.Document, error) {
	doc := did.NewDocument(i.DID())

	// Can only derive a verification method from a multikey verifier. This could
	// be extended in the future.
	mkVerifier, ok := i.Verifier().(multikey.Verifier)
	if !ok {
		return did.Document{}, fmt.Errorf("identity does not have a multikey verifier")
	}
	// Fragment expects a bare name and prepends the "#" itself; passing "#key-0"
	// would serialize as "#%23key-0".
	vm := multikey.DeriveVerificationMethod(doc.Fragment("key-0"), mkVerifier)
	vm.Controller = doc.ID

	if err := doc.VerificationMethods.Add(vm); err != nil {
		return did.Document{}, err
	}

	if err := doc.Authentication.Add(vm); err != nil {
		return did.Document{}, err
	}
	if err := doc.AssertionMethod.Add(vm); err != nil {
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

// NewFromPEMFile creates a new identity from an Ed25519 PEM key file.
func NewFromPEMFile(keyFilePath string) (Identity, error) {
	pem, err := os.ReadFile(keyFilePath)
	if err != nil {
		return Identity{}, fmt.Errorf("failed to read key file: %w", err)
	}
	keySigner, err := DecodeSignerFromPEM(pem)
	if err != nil {
		return Identity{}, fmt.Errorf("failed to decode key from PEM file: %w", err)
	}
	return Identity{Issuer: multikey.KeyIssuer(keySigner)}, nil
}

// NewFromPEMFileWithDID creates a new identity from an Ed25519 PEM key file.
// When serviceDID is provided (e.g., "did:web:upload"), the identity will use
// that DID. Otherwise, it will use the key DID derived from the key.
func NewFromPEMFileWithDID(keyFilePath string, serviceDID string) (Identity, error) {
	keyId, err := NewFromPEMFile(keyFilePath)
	if err != nil {
		return Identity{}, fmt.Errorf("creating identity from PEM file: %w", err)
	}

	// If serviceDID is provided, wrap the signer with the did:web identity
	if serviceDID != "" {
		d, err := did.Parse(serviceDID)
		if err != nil {
			return Identity{}, fmt.Errorf("failed to parse service DID %q: %w", serviceDID, err)
		}

		return Identity{Issuer: multikey.NewIssuer(d, keyId)}, nil
	}

	return Identity{Issuer: keyId.Issuer}, nil
}
