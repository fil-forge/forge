package contentclaims_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/fil-forge/forge/indexing-service/pkg/internal/testutil"
	"github.com/fil-forge/forge/indexing-service/pkg/service/contentclaims"
	"github.com/fil-forge/forge/indexing-service/pkg/types"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
)

func TestWithStore__Find(t *testing.T) {
	storedClaim := testutil.RandomLocationCommitment(t)
	notStoredClaim := testutil.RandomIndexClaim(t)

	storedCid := storedClaim.Link()
	notStoredCid := notStoredClaim.Link()

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
			name:          "Claim stored",
			claimCid:      storedCid,
			expectedClaim: storedClaim,
		},
		{
			name:          "Claim not stored, successful fetch",
			claimCid:      notStoredCid,
			expectedClaim: notStoredClaim,
		},
		{
			name:          "Find error",
			claimCid:      storedCid,
			expectedClaim: nil,
			getErr:        anError,
			expectedErr:   errors.Join(fmt.Errorf("reading from claim store: %w", anError), types.ErrKeyNotFound),
		},
		{
			name:          "underlying find error",
			claimCid:      notStoredCid,
			expectedClaim: nil,
			baseFinder:    &mockFinder{nil, anError},
			expectedErr:   anError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockStore := &MockContentClaimsStore{
				getErr: tc.getErr,
				claims: map[string]ucan.Invocation{
					storedCid.String(): storedClaim,
				},
			}
			finder := tc.baseFinder
			if finder == nil {
				finder = &mockFinder{notStoredClaim, nil}
			}
			cl := contentclaims.WithStore(finder, mockStore)

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

type MockContentClaimsStore struct {
	setErr, getErr error
	claims         map[string]ucan.Invocation
}

var _ types.ContentClaimsStore = &MockContentClaimsStore{}

func (m *MockContentClaimsStore) Get(ctx context.Context, key cid.Cid) (ucan.Invocation, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	claim, exists := m.claims[key.String()]
	if !exists {
		return nil, types.ErrKeyNotFound
	}
	return claim, nil
}

func (m *MockContentClaimsStore) Put(ctx context.Context, key cid.Cid, claim ucan.Invocation) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.claims[key.String()] = claim
	return nil
}
