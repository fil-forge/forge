package contentclaims_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/fil-forge/indexing-service/pkg/internal/testutil"
	"github.com/fil-forge/indexing-service/pkg/service/contentclaims"
	"github.com/fil-forge/indexing-service/pkg/types"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
)

func TestWithCache__Find(t *testing.T) {
	cachedClaim := testutil.RandomLocationCommitment(t)
	notCachedClaim := testutil.RandomIndexClaim(t)

	cachedCid := cachedClaim.Link()
	notCachedCid := notCachedClaim.Link()

	anError := errors.New("something went wrong")
	testCases := []struct {
		name          string
		claimCid      cid.Cid
		setErr        error
		getErr        error
		expectedErr   error
		baseFinder    *mockFinder
		expectedClaim ucan.Invocation
		finalState    map[string]ucan.Invocation
	}{
		{
			name:          "Claim cached",
			claimCid:      cachedCid,
			expectedClaim: cachedClaim,
			finalState: map[string]ucan.Invocation{
				cachedCid.String(): cachedClaim,
			},
		},
		{
			name:          "Claim not cached, successful fetch",
			claimCid:      notCachedCid,
			expectedClaim: notCachedClaim,
			finalState: map[string]ucan.Invocation{
				cachedCid.String():    cachedClaim,
				notCachedCid.String(): notCachedClaim,
			},
		},
		{
			name:          "Find error",
			claimCid:      cachedCid,
			expectedClaim: nil,
			getErr:        anError,
			expectedErr:   fmt.Errorf("reading from claim cache: %w", anError),
			finalState: map[string]ucan.Invocation{
				cachedCid.String(): cachedClaim,
			},
		},
		{
			name:          "Save cache error",
			claimCid:      notCachedCid,
			expectedClaim: nil,
			setErr:        anError,
			expectedErr:   fmt.Errorf("caching claim: %w", anError),
			finalState: map[string]ucan.Invocation{
				cachedCid.String(): cachedClaim,
			},
		},
		{
			name:          "underlying find error",
			claimCid:      notCachedCid,
			expectedClaim: nil,
			baseFinder:    &mockFinder{nil, anError},
			expectedErr:   anError,
			finalState: map[string]ucan.Invocation{
				cachedCid.String(): cachedClaim,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCache := &MockContentClaimsCache{
				setErr: tc.setErr,
				getErr: tc.getErr,
				claims: map[string]ucan.Invocation{
					cachedCid.String(): cachedClaim,
				},
			}
			finder := tc.baseFinder
			if finder == nil {
				finder = &mockFinder{notCachedClaim, nil}
			}
			cl := contentclaims.WithCache(finder, mockCache)

			claim, err := cl.Find(context.Background(), tc.claimCid, testutil.TestURL)
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
			testutil.RequireEqualClaim(t, tc.expectedClaim, claim)
			finalState := tc.finalState
			if finalState == nil {
				finalState = make(map[string]ucan.Invocation)
			}
			require.Equal(t, len(finalState), len(mockCache.claims))
			for c, claim := range mockCache.claims {
				expectedClaim := finalState[c]
				testutil.RequireEqualClaim(t, expectedClaim, claim)
			}
		})
	}
}

type MockContentClaimsCache struct {
	setErr, getErr error
	claims         map[string]ucan.Invocation
}

var _ types.ContentClaimsCache = &MockContentClaimsCache{}

func (m *MockContentClaimsCache) SetExpirable(ctx context.Context, key cid.Cid, expires bool) error {
	return nil
}

func (m *MockContentClaimsCache) Get(ctx context.Context, claimCid cid.Cid) (ucan.Invocation, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	claim, exists := m.claims[claimCid.String()]
	if !exists {
		return nil, types.ErrKeyNotFound
	}
	return claim, nil
}

func (m *MockContentClaimsCache) Set(ctx context.Context, claimCid cid.Cid, claim ucan.Invocation, expires bool) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.claims[claimCid.String()] = claim
	return nil
}

type mockFinder struct {
	claim ucan.Invocation
	err   error
}

func (m *mockFinder) Find(ctx context.Context, id cid.Cid, _ *url.URL) (ucan.Invocation, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.claim == nil || m.claim.Link() != id {
		return nil, types.ErrKeyNotFound
	}
	return m.claim, nil
}
