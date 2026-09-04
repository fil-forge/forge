package blobindex_test

import (
	"bytes"
	"math/rand/v2"
	"testing"

	"github.com/fil-forge/forge/protocol/blobindex"
	"github.com/fil-forge/ucantone/testutil"
	"github.com/ipfs/go-cid"
	multihash "github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"
)

func TestFromToArchive(t *testing.T) {
	testIndexData := map[cid.Cid][]struct {
		digest    multihash.Multihash
		byteRange []int
	}{
		testutil.RandomCID(t): {
			{digest: testutil.RandomDigest(t), byteRange: []int{0, 99}},
			{digest: testutil.RandomDigest(t), byteRange: []int{100, 199}},
			{digest: testutil.RandomDigest(t), byteRange: []int{200, 299}},
		},
		testutil.RandomCID(t): {
			{digest: testutil.RandomDigest(t), byteRange: []int{0, 34}},
			{digest: testutil.RandomDigest(t), byteRange: []int{35, 58}},
		},
	}

	index := blobindex.NewShardedDagIndex(len(testIndexData))
	for shard, slices := range testIndexData {
		for _, slice := range slices {
			index.SetSlice(
				shard.Hash(),
				slice.digest,
				blobindex.Range{
					Start: int64(slice.byteRange[0]),
					End:   int64(slice.byteRange[1]),
				},
			)
		}
	}

	// ensure all the data is in the index
	require.Equal(t, len(testIndexData), index.Shards().Size())
	for shard, slices := range testIndexData {
		require.True(t, index.Shards().Has(shard.Hash()))

		idxSlices := index.Shards().Get(shard.Hash())
		require.Equal(t, len(slices), idxSlices.Size())

		for _, slice := range slices {
			require.True(t, idxSlices.Has(slice.digest))
			byteRange := idxSlices.Get(slice.digest)
			require.Equal(t, int64(slice.byteRange[0]), byteRange.Start)
			require.Equal(t, int64(slice.byteRange[1]), byteRange.End)
		}
	}

	t.Run("round trip", func(t *testing.T) {
		var buf bytes.Buffer
		err := index.Archive(&buf)
		require.NoError(t, err)

		newIndex, err := blobindex.Extract(&buf)
		require.NoError(t, err)

		// ensure all the data is in the new index
		require.Equal(t, index.Shards().Size(), newIndex.Shards().Size())
		for shard, slices := range index.Shards().Iterator() {
			require.True(t, newIndex.Shards().Has(shard))

			newSlices := newIndex.Shards().Get(shard)
			require.Equal(t, slices.Size(), newSlices.Size())

			for slice, byteRange := range slices.Iterator() {
				require.True(t, newSlices.Has(slice))
				newByteRange := newSlices.Get(slice)
				require.Equal(t, byteRange.Start, newByteRange.Start)
				require.Equal(t, byteRange.End, newByteRange.End)
			}
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		index1 := blobindex.NewShardedDagIndex(len(testIndexData))
		for shard, slices := range testIndexData {
			// shuffle the order slices are added
			rand.Shuffle(len(slices), func(i, j int) {
				slices[i], slices[j] = slices[j], slices[i]
			})
			for _, slice := range slices {
				index1.SetSlice(
					shard.Hash(),
					slice.digest,
					blobindex.Range{
						Start: int64(slice.byteRange[0]),
						End:   int64(slice.byteRange[1]),
					},
				)
			}
		}

		var buf1, buf2 bytes.Buffer
		err := index.Archive(&buf1)
		require.NoError(t, err)
		err = index1.Archive(&buf2)
		require.NoError(t, err)

		require.Equal(t, buf1.Bytes(), buf2.Bytes())
	})
}
