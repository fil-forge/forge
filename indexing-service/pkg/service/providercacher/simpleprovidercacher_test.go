package providercacher_test

import (
	"context"
	"slices"
	"sync"
	"testing"

	"github.com/fil-forge/forge/indexing-service/pkg/internal/testutil"
	"github.com/fil-forge/forge/indexing-service/pkg/service/providercacher"
	"github.com/fil-forge/forge/indexing-service/pkg/types"
	"github.com/fil-forge/forge/protocol/blobindex"
	"github.com/ipni/go-libipni/find/model"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"
)

func TestSimpleProviderCacher_CacheProviderForIndexRecords(t *testing.T) {
	ctx := context.Background()

	testProvider := testutil.RandomProviderResult(t)
	testProvider2 := testutil.RandomProviderResult(t)

	shardIndex := blobindex.NewShardedDagIndex(2)

	shardMhs := testutil.RandomMultihashes(t, 2)
	sliceMhs := testutil.RandomMultihashes(t, 6)
	for i := range 2 {
		for j := range 3 {
			shardIndex.SetSlice(shardMhs[i], sliceMhs[i*3+j], blobindex.Range{})
		}
	}
	rootMh := testutil.RandomMultihash(t)
	shardIndex.SetSlice(shardMhs[0], rootMh, blobindex.Range{})

	shardIndex2 := blobindex.NewShardedDagIndex(2)
	for j := range 2 {
		shardIndex2.SetSlice(shardMhs[0], sliceMhs[j], blobindex.Range{})
	}
	rootMh2 := testutil.RandomMultihash(t)
	shardIndex2.SetSlice(shardMhs[0], rootMh2, blobindex.Range{})

	evensFilled := func() map[string][]model.ProviderResult {
		starter := make(map[string][]model.ProviderResult)
		for i, sliceMh := range sliceMhs {
			if i%2 == 0 {
				starter[sliceMh.String()] = []model.ProviderResult{testProvider}
			}
		}
		return starter
	}

	testCases := []struct {
		name         string
		provider     model.ProviderResult
		index        blobindex.ShardedDagIndex
		getErr       error
		setErr       error
		initialStore map[string][]model.ProviderResult
		expectedErr  error
		testStore    func(t *testing.T, store map[string][]model.ProviderResult)
	}{
		{
			name:        "Cache new provider",
			provider:    testProvider,
			index:       shardIndex,
			expectedErr: nil,
			testStore: func(t *testing.T, store map[string][]model.ProviderResult) {
				require.Len(t, store, 7)
				for _, sliceMh := range sliceMhs {
					require.Equal(t, store[sliceMh.String()], []model.ProviderResult{testProvider})
				}
			},
		},
		{
			name:         "Cache provider already present",
			provider:     testProvider,
			index:        shardIndex,
			initialStore: evensFilled(),
			expectedErr:  nil,
			testStore: func(t *testing.T, store map[string][]model.ProviderResult) {
				require.Len(t, store, 7)
				for _, sliceMh := range sliceMhs {
					require.Equal(t, store[sliceMh.String()], []model.ProviderResult{testProvider})
				}
			},
		},
		{
			name:         "Cache another provider on top",
			provider:     testProvider2,
			index:        shardIndex,
			initialStore: evensFilled(),
			expectedErr:  nil,
			testStore: func(t *testing.T, store map[string][]model.ProviderResult) {
				require.Len(t, store, 7)
				for i, sliceMh := range sliceMhs {
					expected := []model.ProviderResult{testProvider2}
					if i%2 == 0 {
						expected = []model.ProviderResult{testProvider, testProvider2}
					}
					require.Equal(t, store[sliceMh.String()], expected)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			initialStore := tc.initialStore
			if initialStore == nil {
				initialStore = make(map[string][]model.ProviderResult)
			}
			mockStore := &MockProviderStore{
				setErr: tc.setErr,
				getErr: tc.getErr,
				store:  initialStore,
			}

			cacher := providercacher.NewSimpleProviderCacher(mockStore)

			err := cacher.CacheProviderForIndexRecords(ctx, tc.provider, tc.index)
			if tc.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expectedErr.Error())
			}
			if tc.testStore != nil {
				tc.testStore(t, mockStore.store)
			}
		})
	}
}

// Simulate a 10k NFT - a directory with 10k image/metadata files
func TestSimpleProviderCacher_10kNFT(t *testing.T) {
	ctx := context.Background()

	prov := testutil.RandomProviderResult(t)
	idx := blobindex.NewShardedDagIndex(2)

	shardDigests := testutil.RandomMultihashes(t, 2)
	sliceDigests := make([]multihash.Multihash, 0, 10_000)
	for range 10_000 {
		for {
			digest := testutil.RandomMultihash(t)
			if slices.ContainsFunc(sliceDigests, func(s multihash.Multihash) bool { return s.String() == digest.String() }) {
				continue
			}
			sliceDigests = append(sliceDigests, digest)
			break
		}
	}

	for i := range 2 {
		for j := range 5_000 {
			idx.SetSlice(shardDigests[i], sliceDigests[i*5_000+j], blobindex.Range{})
		}
	}
	rootMh := testutil.RandomMultihash(t)
	idx.SetSlice(shardDigests[0], rootMh, blobindex.Range{})

	mockStore := &MockProviderStore{store: map[string][]model.ProviderResult{}}

	cacher := providercacher.NewSimpleProviderCacher(mockStore)
	err := cacher.CacheProviderForIndexRecords(ctx, prov, idx)
	require.NoError(t, err)
}

type MockProviderStore struct {
	setErr, getErr error
	store          map[string][]model.ProviderResult
	mutex          sync.RWMutex
}

var _ types.ProviderStore = &MockProviderStore{}

func (m *MockProviderStore) Members(ctx context.Context, hash multihash.Multihash) ([]model.ProviderResult, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	results, exists := m.store[hash.String()]
	if !exists {
		return nil, types.ErrKeyNotFound
	}
	return results, nil
}

func (m *MockProviderStore) Add(ctx context.Context, hash multihash.Multihash, providers ...model.ProviderResult) (uint64, error) {
	written := uint64(0)
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for _, provider := range providers {
		existing := m.store[hash.String()]
		if !slices.ContainsFunc(existing, func(p model.ProviderResult) bool { return p.Equal(provider) }) {
			m.store[hash.String()] = append(existing, provider)
			written++
		}
	}
	return written, nil
}

func (m *MockProviderStore) SetExpirable(ctx context.Context, key multihash.Multihash, expires bool) error {
	return nil
}

func (m *MockProviderStore) Batch() types.ValueSetCacheBatcher[multihash.Multihash, model.ProviderResult] {
	return &MockBatcher{store: m}
}

type MockCommand struct {
	op     string
	key    multihash.Multihash
	values []model.ProviderResult
	expire bool
}

type MockBatcher struct {
	store    *MockProviderStore
	commands []MockCommand
}

func (mb *MockBatcher) Add(ctx context.Context, key multihash.Multihash, newProviders ...model.ProviderResult) error {
	mb.commands = append(mb.commands, MockCommand{op: "add", key: key, values: newProviders})
	return nil
}

func (mb *MockBatcher) SetExpirable(ctx context.Context, key multihash.Multihash, expires bool) error {
	mb.commands = append(mb.commands, MockCommand{op: "setExpirable", key: key, expire: expires})
	return nil
}

func (mb *MockBatcher) Commit(ctx context.Context) error {
	for _, c := range mb.commands {
		if c.op == "add" {
			_, err := mb.store.Add(ctx, c.key, c.values...)
			if err != nil {
				return err
			}
		} else if c.op == "setExpirable" {
			err := mb.store.SetExpirable(ctx, c.key, c.expire)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
