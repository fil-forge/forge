package testutil

import (
	"testing"

	"github.com/fil-forge/forge/internal/identity"
	"github.com/fil-forge/ucantone/testutil"
	"github.com/ipfs/go-cid"
)

var (
	Alice                = testutil.Alice
	Bob                  = testutil.Bob
	Carol                = testutil.Carol
	Mallory              = testutil.Mallory
	Service              = testutil.Service
	WebService           = identity.Identity{Issuer: testutil.WebService}
	WebServiceSigner     = testutil.WebServiceSigner
	RandomBytes          = testutil.RandomBytes
	RandomDID            = testutil.RandomDID
	RandomDigest         = testutil.RandomDigest
	RandomIssuer         = testutil.RandomIssuer
	RandomMultikeyIssuer = testutil.RandomMultikeyIssuer
)

func RandomCID(t *testing.T) cid.Cid {
	return cid.NewCidV1(cid.Raw, RandomDigest(t))
}

func Must[T any](val T, err error) func(*testing.T) T {
	return testutil.Must(val, err)
}
