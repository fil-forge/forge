package contentclaims_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fil-forge/forge/indexing-service/pkg/internal/testutil"
	"github.com/fil-forge/forge/indexing-service/pkg/service/contentclaims"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/ipfs/go-cid"
	multihash "github.com/multiformats/go-multihash/core"
	"github.com/stretchr/testify/require"
)

func TestIdentityCidFinder__Find(t *testing.T) {
	identityCidClaim := testutil.RandomLocationCommitment(t)
	notIdentityCidClaim := testutil.RandomIndexClaim(t)

	identityCiddata := identityCidClaim.Bytes()

	identityCid := testutil.Must(cid.Prefix{
		Version:  1,
		Codec:    cid.Raw,
		MhType:   multihash.IDENTITY,
		MhLength: len(identityCiddata),
	}.Sum(identityCiddata))(t)
	notIdentityCid := notIdentityCidClaim.Link()

	anError := errors.New("something went wrong")
	testCases := []struct {
		name          string
		claimCid      cid.Cid
		getErr        error
		expectedErr   error
		baseFinder    *mockFinder
		expectedClaim ucan.Invocation
	}{
		{
			name:          "identity cid",
			claimCid:      identityCid,
			expectedClaim: identityCidClaim,
		},
		{
			name:          "not identity cid, successful fetch",
			claimCid:      notIdentityCid,
			expectedClaim: notIdentityCidClaim,
		},
		{
			name:          "underlying find error",
			claimCid:      notIdentityCid,
			expectedClaim: nil,
			baseFinder:    &mockFinder{nil, anError},
			expectedErr:   anError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			finder := tc.baseFinder
			if finder == nil {
				finder = &mockFinder{notIdentityCidClaim, nil}
			}
			cl := contentclaims.WithIdentityCids(finder)

			claim, err := cl.Find(context.Background(), tc.claimCid, testutil.TestURL)
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
			testutil.RequireEqualClaim(t, tc.expectedClaim, claim)
		})
	}
}
