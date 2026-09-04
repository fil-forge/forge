package blobindexlookup_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/fil-forge/forge/indexing-service/pkg/internal/testutil"
	"github.com/fil-forge/forge/indexing-service/pkg/service/blobindexlookup"
	"github.com/fil-forge/forge/indexing-service/pkg/service/providercacher"
	"github.com/fil-forge/forge/indexing-service/pkg/types"
	"github.com/fil-forge/forge/protocol/blobindex"
	"github.com/ipni/go-libipni/find/model"
	"github.com/stretchr/testify/require"
)

func TestWithCache__Find(t *testing.T) {
	cachedContextID := testutil.RandomBytes(t, 16)
	notCachedContextID := testutil.RandomBytes(t, 16)
	_, cachedIndex := testutil.RandomShardedDagIndex(t, 32)
	_, notCachedIndex := testutil.RandomShardedDagIndex(t, 32)

	provider := testutil.RandomProviderResult(t)

	anError := errors.New("something went wrong")
	testCases := []struct {
		name           string
		contextID      types.EncodedContextID
		setErr         error
		getErr         error
		expectedErr    error
		baseLookup     *mockBlobIndexLookup
		providerCacher *mockCachingQueue
		expectedIndex  blobindex.ShardedDagIndex
		finalState     map[string]blobindex.ShardedDagIndex
	}{
		{
			name:          "Index cached",
			contextID:     cachedContextID,
			expectedIndex: cachedIndex,
			finalState: map[string]blobindex.ShardedDagIndex{
				string(cachedContextID): cachedIndex,
			},
		},
		{
			name:          "Index not cached, successful fetch",
			contextID:     notCachedContextID,
			expectedIndex: notCachedIndex,
			finalState: map[string]blobindex.ShardedDagIndex{
				string(cachedContextID):    cachedIndex,
				string(notCachedContextID): notCachedIndex,
			},
		},
		{
			name:          "Lookup error",
			contextID:     cachedContextID,
			expectedIndex: nil,
			getErr:        anError,
			expectedErr:   fmt.Errorf("reading from index cache: %w", anError),
			finalState: map[string]blobindex.ShardedDagIndex{
				string(cachedContextID): cachedIndex,
			},
		},
		{
			name:          "Save cache error",
			contextID:     notCachedContextID,
			expectedIndex: nil,
			setErr:        anError,
			expectedErr:   fmt.Errorf("caching fetched index: %w", anError),
			finalState: map[string]blobindex.ShardedDagIndex{
				string(cachedContextID): cachedIndex,
			},
		},
		{
			name:          "underlying lookup error",
			contextID:     notCachedContextID,
			expectedIndex: nil,
			baseLookup:    &mockBlobIndexLookup{nil, anError},
			expectedErr:   fmt.Errorf("fetching underlying index: %w", anError),
			finalState: map[string]blobindex.ShardedDagIndex{
				string(cachedContextID): cachedIndex,
			},
		},
		{
			name:           "provider cacher error",
			contextID:      notCachedContextID,
			expectedIndex:  nil,
			providerCacher: &mockCachingQueue{anError},
			expectedErr:    fmt.Errorf("queueing provider caching for index failed: %w", anError),
			finalState: map[string]blobindex.ShardedDagIndex{
				string(cachedContextID):    cachedIndex,
				string(notCachedContextID): notCachedIndex,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockStore := &MockShardedDagIndexStore{
				setErr: tc.setErr,
				getErr: tc.getErr,
				indexes: map[string]blobindex.ShardedDagIndex{
					string(cachedContextID): cachedIndex,
				},
			}
			lookup := tc.baseLookup
			if lookup == nil {
				lookup = &mockBlobIndexLookup{notCachedIndex, nil}
			}
			providerCacher := tc.providerCacher
			if providerCacher == nil {
				providerCacher = &mockCachingQueue{nil}
			}
			cl := blobindexlookup.WithCache(lookup, mockStore, providerCacher)

			req := types.RetrievalRequest{URL: *testutil.TestURL}
			index, err := cl.Find(context.Background(), tc.contextID, provider, req)
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
			testutil.RequireEqualIndex(t, tc.expectedIndex, index)
			finalState := tc.finalState
			if finalState == nil {
				finalState = make(map[string]blobindex.ShardedDagIndex)
			}
			require.Equal(t, len(finalState), len(mockStore.indexes))
			for c, index := range mockStore.indexes {
				expectedIndex := finalState[c]
				testutil.RequireEqualIndex(t, expectedIndex, index)
			}
		})
	}
}

type MockShardedDagIndexStore struct {
	setErr, getErr error
	indexes        map[string]blobindex.ShardedDagIndex
}

var _ types.ShardedDagIndexStore = &MockShardedDagIndexStore{}

func (m *MockShardedDagIndexStore) SetExpirable(ctx context.Context, contextID types.EncodedContextID, expires bool) error {
	return nil
}

func (m *MockShardedDagIndexStore) Get(ctx context.Context, contextID types.EncodedContextID) (blobindex.ShardedDagIndex, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	index, exists := m.indexes[string(contextID)]
	if !exists {
		return nil, types.ErrKeyNotFound
	}
	return index, nil
}

func (m *MockShardedDagIndexStore) Set(ctx context.Context, contextID types.EncodedContextID, index blobindex.ShardedDagIndex, expire bool) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.indexes[string(contextID)] = index
	return nil
}

type mockBlobIndexLookup struct {
	index blobindex.ShardedDagIndex
	err   error
}

func (m *mockBlobIndexLookup) Find(ctx context.Context, contextID types.EncodedContextID, provider model.ProviderResult, req types.RetrievalRequest) (blobindex.ShardedDagIndex, error) {
	return m.index, m.err
}

type mockCachingQueue struct {
	err error
}

func (m *mockCachingQueue) Queue(ctx context.Context, job providercacher.ProviderCachingJob) error {
	return m.err
}
