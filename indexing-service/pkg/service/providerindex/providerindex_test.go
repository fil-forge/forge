package providerindex

import (
	"context"
	"errors"
	"iter"
	"slices"
	"testing"

	"github.com/fil-forge/go-ipni-tools/pkg/metadata"
	"github.com/fil-forge/go-ipni-tools/pkg/publisher"
	"github.com/fil-forge/indexing-service/pkg/internal/extmocks"
	"github.com/fil-forge/indexing-service/pkg/internal/testutil"
	"github.com/fil-forge/indexing-service/pkg/types"
	"github.com/ipni/go-libipni/find/model"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGetProviderResults(t *testing.T) {
	t.Run("results found in the cache", func(t *testing.T) {
		mockStore := types.NewMockProviderStore(t)
		mockNoProviderStore := types.NewMockNoProviderStore(t)
		mockIpniFinder := extmocks.NewMockIpniFinder(t)
		mockIpniPublisher := extmocks.NewMockIpniPublisher(t)

		providerIndex := New(mockStore, mockNoProviderStore, mockIpniFinder, mockIpniPublisher)

		someHash := testutil.RandomMultihash(t)
		expectedResult := testutil.RandomLocationCommitmentProviderResult(t)

		mockStore.EXPECT().Members(extmocks.AnyContext, someHash).Return([]model.ProviderResult{expectedResult}, nil)

		results, err := providerIndex.getProviderResults(context.Background(), someHash, []multicodec.Code{metadata.LocationCommitmentID})

		require.NoError(t, err)
		require.Equal(t, []model.ProviderResult{expectedResult}, results)
	})

	t.Run("cache hit but doesn't match target claims, falls through to IPNI", func(t *testing.T) {
		mockStore := types.NewMockProviderStore(t)
		mockNoProviderStore := types.NewMockNoProviderStore(t)
		mockIpniFinder := extmocks.NewMockIpniFinder(t)
		mockIpniPublisher := extmocks.NewMockIpniPublisher(t)

		providerIndex := New(mockStore, mockNoProviderStore, mockIpniFinder, mockIpniPublisher)

		existingResult := testutil.RandomIndexClaimProviderResult(t)
		someHash := testutil.RandomMultihash(t)
		expectedResult := testutil.RandomLocationCommitmentProviderResult(t)
		ipniFinderResponse := &model.FindResponse{
			MultihashResults: []model.MultihashResult{
				{
					Multihash:       someHash,
					ProviderResults: []model.ProviderResult{expectedResult},
				},
			},
		}
		targetClaim := []multicodec.Code{metadata.LocationCommitmentID}

		mockStore.EXPECT().Members(extmocks.AnyContext, someHash).Return([]model.ProviderResult{existingResult}, nil)
		mockNoProviderStore.EXPECT().Members(extmocks.AnyContext, someHash).Return(nil, types.ErrKeyNotFound)
		mockIpniFinder.EXPECT().Find(extmocks.AnyContext, someHash).Return(ipniFinderResponse, nil)
		mockStore.EXPECT().Add(extmocks.AnyContext, someHash, expectedResult).Return(1, nil)
		mockStore.EXPECT().SetExpirable(extmocks.AnyContext, someHash, true).Return(nil)

		results, err := providerIndex.getProviderResults(context.Background(), someHash, targetClaim)

		require.NoError(t, err)
		require.Equal(t, []model.ProviderResult{expectedResult}, results)
	})

	t.Run("returns an empty slice when no results are found, no providers record gets cached", func(t *testing.T) {
		mockStore := types.NewMockProviderStore(t)
		mockNoProviderStore := types.NewMockNoProviderStore(t)
		mockIpniFinder := extmocks.NewMockIpniFinder(t)
		mockIpniPublisher := extmocks.NewMockIpniPublisher(t)

		providerIndex := New(mockStore, mockNoProviderStore, mockIpniFinder, mockIpniPublisher)

		someHash := testutil.RandomMultihash(t)

		mockStore.EXPECT().Members(extmocks.AnyContext, someHash).Return(nil, types.ErrKeyNotFound)
		mockNoProviderStore.EXPECT().Members(extmocks.AnyContext, someHash).Return(nil, types.ErrKeyNotFound)
		mockIpniFinder.EXPECT().Find(extmocks.AnyContext, someHash).Return(&model.FindResponse{}, nil)
		mockNoProviderStore.EXPECT().Add(extmocks.AnyContext, someHash, multicodec.Code(0)).Return(1, nil)
		mockNoProviderStore.EXPECT().SetExpirable(extmocks.AnyContext, someHash, true).Return(nil)

		results, err := providerIndex.getProviderResults(context.Background(), someHash, []multicodec.Code{0})

		require.NoError(t, err)
		require.Empty(t, results)
	})

	t.Run("error in cache returns error", func(t *testing.T) {
		mockStore := types.NewMockProviderStore(t)
		mockNoProviderStore := types.NewMockNoProviderStore(t)
		mockIpniFinder := extmocks.NewMockIpniFinder(t)
		mockIpniPublisher := extmocks.NewMockIpniPublisher(t)

		providerIndex := New(mockStore, mockNoProviderStore, mockIpniFinder, mockIpniPublisher)

		someHash := testutil.RandomMultihash(t)

		mockStore.EXPECT().Members(extmocks.AnyContext, someHash).Return(nil, errors.New("some error"))

		_, err := providerIndex.getProviderResults(context.Background(), someHash, []multicodec.Code{0})

		require.Error(t, err)
	})

	t.Run("error from IPNI is logged and no providers are cached", func(t *testing.T) {
		mockStore := types.NewMockProviderStore(t)
		mockNoProviderStore := types.NewMockNoProviderStore(t)
		mockIpniFinder := extmocks.NewMockIpniFinder(t)
		mockIpniPublisher := extmocks.NewMockIpniPublisher(t)

		providerIndex := New(mockStore, mockNoProviderStore, mockIpniFinder, mockIpniPublisher)

		someHash := testutil.RandomMultihash(t)

		mockStore.EXPECT().Members(extmocks.AnyContext, someHash).Return(nil, types.ErrKeyNotFound)
		mockNoProviderStore.EXPECT().Members(extmocks.AnyContext, someHash).Return(nil, types.ErrKeyNotFound)
		mockIpniFinder.EXPECT().Find(extmocks.AnyContext, someHash).Return(nil, errors.New("some error"))
		mockNoProviderStore.EXPECT().Add(extmocks.AnyContext, someHash, multicodec.Code(0)).Return(1, nil)
		mockNoProviderStore.EXPECT().SetExpirable(extmocks.AnyContext, someHash, true).Return(nil)

		results, err := providerIndex.getProviderResults(context.Background(), someHash, []multicodec.Code{0})

		require.NoError(t, err)
		require.Empty(t, results)
	})
}

func TestPublish(t *testing.T) {
	t.Run("allow skip publish existing advert", func(t *testing.T) {
		mockStore := types.NewMockProviderStore(t)
		mockBatcher := types.NewMockValueSetCacheBatcher[multihash.Multihash, model.ProviderResult](t)
		mockNoProviderStore := types.NewMockNoProviderStore(t)
		mockIpniFinder := extmocks.NewMockIpniFinder(t)
		mockIpniPublisher := extmocks.NewMockIpniPublisher(t)

		providerIndex := New(mockStore, mockNoProviderStore, mockIpniFinder, mockIpniPublisher)

		result := testutil.RandomLocationCommitmentProviderResult(t)
		provider := *result.Provider
		contextID := string(result.ContextID)
		digest := testutil.RandomMultihash(t)
		anyDigestSeq := mock.MatchedBy(func(digests iter.Seq[multihash.Multihash]) bool {
			return true
		})
		meta := metadata.MetadataContext.New()
		err := meta.UnmarshalBinary(result.Metadata)
		require.NoError(t, err)

		mockStore.EXPECT().Batch().Return(mockBatcher)
		mockBatcher.EXPECT().Add(extmocks.AnyContext, digest, result).Return(nil)
		mockBatcher.EXPECT().SetExpirable(extmocks.AnyContext, digest, false).Return(nil)
		mockBatcher.EXPECT().Commit(extmocks.AnyContext).Return(nil)
		mockIpniPublisher.EXPECT().Publish(extmocks.AnyContext, provider, contextID, anyDigestSeq, meta).Return(publisher.ErrAlreadyAdvertised)

		err = providerIndex.Publish(context.Background(), provider, contextID, slices.Values([]multihash.Multihash{digest}), meta)
		require.NoError(t, err)
	})
}
