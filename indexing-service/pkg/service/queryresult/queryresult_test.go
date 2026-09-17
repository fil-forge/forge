package queryresult

import (
	"bytes"
	"testing"

	"github.com/fil-forge/indexing-service/pkg/internal/testutil"
	"github.com/fil-forge/indexing-service/pkg/types"
	"github.com/fil-forge/libforge/blobindex"
	"github.com/fil-forge/libforge/bytemap"
	"github.com/fil-forge/libforge/commands"
	"github.com/fil-forge/libforge/commands/assert"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/fil-forge/ucantone/ucan/invocation"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
)

func TestBuildCompressed(t *testing.T) {
	t.Run("compresses with matching index entry", func(t *testing.T) {
		principal := testutil.RandomIssuer(t)
		targetMh := testutil.RandomMultihash(t)
		index := blobindex.NewShardedDagIndex(1)
		shardMh := testutil.RandomMultihash(t)

		targetRange := blobindex.Range{Start: 100, End: 149}
		index.SetSlice(shardMh, targetMh, targetRange)

		for i := 0; i < 5; i++ {
			index.SetSlice(shardMh, testutil.RandomMultihash(t), blobindex.Range{
				Start: int64(200 + i*100),
				End:   int64(200 + i*100 + 74),
			})
		}

		indexHash := shardMh

		shardLength := uint64(5000)
		shardClaim, err := assert.Location.Invoke(
			principal,
			principal.DID(),
			&assert.LocationArguments{
				Space:    principal.DID(),
				Content:  shardMh,
				Location: []commands.CborURL{commands.CborURL(*testutil.TestURL)},
				Range: &assert.Range{
					Start: 1000,
					End:   &shardLength,
				},
			},
		)
		require.NoError(t, err)

		otherShardLength := uint64(8000)
		otherShardMH := testutil.RandomMultihash(t)
		otherShardClaim, err := assert.Location.Invoke(
			principal,
			principal.DID(),
			&assert.LocationArguments{
				Space:    principal.DID(),
				Content:  otherShardMH,
				Location: []commands.CborURL{commands.CborURL(*testutil.TestURL)},
				Range: &assert.Range{
					Start: 1000,
					End:   &otherShardLength,
				},
			},
		)
		require.NoError(t, err)

		claims := map[cid.Cid]ucan.Invocation{
			otherShardClaim.Link(): otherShardClaim,
			shardClaim.Link():      shardClaim,
		}

		indexes := bytemap.NewByteMap[types.EncodedContextID, blobindex.ShardedDagIndex](1)
		indexContextID, err := types.ContextID{Hash: indexHash}.ToEncoded()
		require.NoError(t, err)
		indexes.Set(indexContextID, index)

		result, err := BuildCompressed(targetMh, principal, claims, indexes)
		require.NoError(t, err)

		resultClaims := result.Claims()
		require.Len(t, resultClaims, 1, "should have exactly one claim")

		resultIndexes := result.Indexes()
		require.Len(t, resultIndexes, 0, "should have no indexes")

		var compressedBlock *types.Block
		for _, blk := range result.Blocks() {
			if blk.Link == resultClaims[0] {
				b := blk
				compressedBlock = &b
				break
			}
		}
		require.NotNil(t, compressedBlock, "should find the compressed claim block")

		decoded, err := invocation.Decode(compressedBlock.Data)
		require.NoError(t, err)
		require.Equal(t, assert.Location.Command, decoded.Command())

		var args assert.LocationArguments
		require.NoError(t, args.UnmarshalCBOR(bytes.NewReader(decoded.ArgumentsBytes())))
		require.True(t, bytes.Equal(args.Content, targetMh))
		require.NotNil(t, args.Range)
		require.Equal(t, uint64(1000)+uint64(targetRange.Start), args.Range.Start)
		require.NotNil(t, args.Range.End)
		require.Equal(t, uint64(1000)+uint64(targetRange.End), *args.Range.End)
	})

	t.Run("doesn't fail when matching location claim doesn't have a range", func(t *testing.T) {
		principal := testutil.RandomIssuer(t)
		targetMh := testutil.RandomMultihash(t)
		index := blobindex.NewShardedDagIndex(1)
		shardMh := testutil.RandomMultihash(t)

		targetRange := blobindex.Range{Start: 100, End: 149}
		index.SetSlice(shardMh, targetMh, targetRange)

		for i := 0; i < 5; i++ {
			index.SetSlice(shardMh, testutil.RandomMultihash(t), blobindex.Range{
				Start: int64(200 + i*100),
				End:   int64(200 + i*100 + 74),
			})
		}

		indexHash := shardMh

		shardClaim, err := assert.Location.Invoke(
			principal,
			principal.DID(),
			&assert.LocationArguments{
				Space:    principal.DID(),
				Content:  shardMh,
				Location: []commands.CborURL{commands.CborURL(*testutil.TestURL)},
				Range:    nil,
			},
		)
		require.NoError(t, err)

		claims := map[cid.Cid]ucan.Invocation{
			shardClaim.Link(): shardClaim,
		}

		indexes := bytemap.NewByteMap[types.EncodedContextID, blobindex.ShardedDagIndex](1)
		indexContextID, err := types.ContextID{Hash: indexHash}.ToEncoded()
		require.NoError(t, err)
		indexes.Set(indexContextID, index)

		result, err := BuildCompressed(targetMh, principal, claims, indexes)
		require.NoError(t, err)

		resultClaims := result.Claims()
		require.Len(t, resultClaims, 1, "should have exactly one claim")

		resultIndexes := result.Indexes()
		require.Len(t, resultIndexes, 0, "should have no indexes")

		var compressedBlock *types.Block
		for _, blk := range result.Blocks() {
			if blk.Link == resultClaims[0] {
				b := blk
				compressedBlock = &b
				break
			}
		}
		require.NotNil(t, compressedBlock)

		decoded, err := invocation.Decode(compressedBlock.Data)
		require.NoError(t, err)
		var args assert.LocationArguments
		require.NoError(t, args.UnmarshalCBOR(bytes.NewReader(decoded.ArgumentsBytes())))
		require.True(t, bytes.Equal(args.Content, targetMh))
		require.NotNil(t, args.Range)
		require.Equal(t, uint64(targetRange.Start), args.Range.Start)
		require.NotNil(t, args.Range.End)
		require.Equal(t, uint64(targetRange.End), *args.Range.End)
	})

	t.Run("returns regular result when no matching index entry", func(t *testing.T) {
		principal := testutil.RandomIssuer(t)
		targetMh := testutil.RandomMultihash(t)
		index := blobindex.NewShardedDagIndex(1)

		shardMh := testutil.RandomMultihash(t)
		for i := 0; i < 5; i++ {
			index.SetSlice(shardMh, testutil.RandomMultihash(t), blobindex.Range{
				Start: int64(100 + i*100),
				End:   int64(100 + i*100 + 49),
			})
		}

		indexHash := shardMh

		locationClaim := testutil.RandomLocationCommitment(t)
		indexClaim := testutil.RandomIndexClaim(t)
		claims := map[cid.Cid]ucan.Invocation{
			locationClaim.Link(): locationClaim,
			indexClaim.Link():    indexClaim,
		}

		indexes := bytemap.NewByteMap[types.EncodedContextID, blobindex.ShardedDagIndex](1)
		indexContextID, err := types.ContextID{Hash: indexHash}.ToEncoded()
		require.NoError(t, err)
		indexes.Set(indexContextID, index)

		result, err := BuildCompressed(targetMh, principal, claims, indexes)
		require.NoError(t, err)

		resultClaims := result.Claims()
		require.Len(t, resultClaims, 2, "should have both original claims")

		resultIndexes := result.Indexes()
		require.Len(t, resultIndexes, 1, "should have the original index")
	})

	t.Run("returns regular result when no indexes", func(t *testing.T) {
		principal := testutil.RandomIssuer(t)
		targetMh := testutil.RandomMultihash(t)

		locationClaim := testutil.RandomLocationCommitment(t)
		claims := map[cid.Cid]ucan.Invocation{
			locationClaim.Link(): locationClaim,
		}

		indexes := bytemap.NewByteMap[types.EncodedContextID, blobindex.ShardedDagIndex](-1)

		result, err := BuildCompressed(targetMh, principal, claims, indexes)
		require.NoError(t, err)

		resultClaims := result.Claims()
		require.Len(t, resultClaims, 1)

		resultIndexes := result.Indexes()
		require.Len(t, resultIndexes, 0)
	})
}
