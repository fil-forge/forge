//go:build !codegen

package queryresult

import (
	"bytes"
	"fmt"
	"io"
	"maps"
	"slices"

	"github.com/fil-forge/automobile"
	"github.com/fil-forge/forge/indexing-service/pkg/types"
	"github.com/fil-forge/forge/protocol/blobindex"
	"github.com/fil-forge/forge/protocol/bytemap"
	"github.com/fil-forge/forge/protocol/commands/assert"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/fil-forge/ucantone/ucan/invocation"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"
	mh "github.com/multiformats/go-multihash"
	multihash "github.com/multiformats/go-multihash/core"
)

type queryResult struct {
	root  types.Block
	model *QueryResult0_1
	blks  []types.Block
}

var _ types.QueryResult = (*queryResult)(nil)

func (q *queryResult) Blocks() []types.Block {
	return q.blks
}

func (q *queryResult) Claims() []cid.Cid {
	return q.model.Claims
}

func (q *queryResult) Indexes() []cid.Cid {
	return slices.Collect(maps.Values(q.model.Indexes))
}

func (q *queryResult) Root() types.Block {
	return q.root
}

func Extract(r io.Reader) (types.QueryResult, error) {
	roots, carBlocks, err := automobile.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("extracting car: %w", err)
	}

	if len(roots) != 1 {
		return nil, types.ErrWrongRootCount
	}

	var rootBlock *types.Block
	blocks := make([]types.Block, len(carBlocks))
	for i, blk := range carBlocks {
		blocks[i] = types.Block(blk)
		if blk.Link == roots[0] {
			b := types.Block(blk)
			rootBlock = &b
		}
	}
	if rootBlock == nil {
		return nil, types.ErrNoRootBlock
	}

	var queryResultModel QueryResult
	if err := queryResultModel.UnmarshalCBOR(bytes.NewReader(rootBlock.Data)); err != nil {
		return nil, fmt.Errorf("decoding query result: %w", err)
	}
	return &queryResult{root: *rootBlock, model: queryResultModel.Result0_1, blks: blocks}, nil
}

// Build generates a new encodable QueryResult
func Build(claims map[cid.Cid]ucan.Invocation, indexes bytemap.ByteMap[types.EncodedContextID, blobindex.ShardedDagIndex]) (types.QueryResult, error) {
	blocks := make([]types.Block, 0, len(claims)+indexes.Size()+1)

	cls := []cid.Cid{}
	for _, claim := range claims {
		cls = append(cls, claim.Link())
		blocks = append(blocks, types.Block{
			Link: claim.Link(),
			Data: claim.Bytes(),
		})
	}

	indexesModel := map[string]cid.Cid{}
	if indexes.Size() > 0 {
		for contextID, index := range indexes.Iterator() {
			var buf bytes.Buffer
			if err := blobindex.Archive(index, &buf); err != nil {
				return nil, fmt.Errorf("archiving index: %w", err)
			}
			indexLink, err := cid.Prefix{
				Version:  1,
				Codec:    uint64(multicodec.Car),
				MhType:   multihash.SHA2_256,
				MhLength: -1,
			}.Sum(buf.Bytes())
			if err != nil {
				return nil, err
			}
			indexesModel[string(contextID)] = indexLink
			blocks = append(blocks, types.Block{Link: indexLink, Data: buf.Bytes()})
		}
	}

	queryResultModel := QueryResult{
		Result0_1: &QueryResult0_1{
			Claims:  cls,
			Indexes: indexesModel,
		},
	}

	var rootBuf bytes.Buffer
	if err := queryResultModel.MarshalCBOR(&rootBuf); err != nil {
		return nil, fmt.Errorf("encoding query result: %w", err)
	}
	rootLink, err := cid.V1Builder{
		Codec:    cid.DagCBOR,
		MhType:   multihash.SHA2_256,
		MhLength: -1,
	}.Sum(rootBuf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("building root CID: %w", err)
	}
	rootBlock := types.Block{Link: rootLink, Data: rootBuf.Bytes()}
	blocks = append(blocks, rootBlock)

	return &queryResult{root: rootBlock, model: queryResultModel.Result0_1, blks: blocks}, nil
}

// BuildCompressed returns a QueryResult that, when there is a matching index entry for the
// targetMh, replaces the full index with a single location claim for the targetMh
func BuildCompressed(targetMh mh.Multihash, principal ucan.Issuer, claims map[cid.Cid]ucan.Invocation, indexes bytemap.ByteMap[types.EncodedContextID, blobindex.ShardedDagIndex]) (types.QueryResult, error) {

	// our goal here is to remove indexes from the query result if there are any
	// if there are no indexes, we can just build the regular query result
	if indexes.Size() == 0 {
		return Build(claims, indexes)
	}

	for _, index := range indexes.Iterator() {
		for shardHash, shard := range index.Shards().Iterator() {
			if shard.Has(targetMh) {
				pos := shard.Get(targetMh)
				hasLocation := false
				var locClaim assert.LocationArguments
				var expiration *ucan.UnixTimestamp
				for _, claim := range claims {
					if claim.Command() != assert.Location.Command {
						continue
					}
					if err := locClaim.UnmarshalCBOR(bytes.NewReader(claim.ArgumentsBytes())); err != nil {
						continue
					}
					if !bytes.Equal(locClaim.Content, shardHash) {
						continue
					}
					hasLocation = true
					expiration = claim.Expiration()
					break
				}
				if !hasLocation {
					continue
				}

				start := uint64(pos.Start)
				end := uint64(pos.End)
				if locClaim.Range != nil {
					start = locClaim.Range.Start + uint64(pos.Start)
					if locClaim.Range.End != nil {
						end = locClaim.Range.Start + uint64(pos.End)
					}
				}
				newArgs := assert.LocationArguments{
					Content:  targetMh,
					Location: locClaim.Location,
					Range:    &assert.Range{Start: start, End: &end},
					Space:    locClaim.Space,
				}
				var opts = []invocation.Option{
					invocation.WithAudience(principal.DID()),
				}
				if expiration != nil {
					opts = append(opts, invocation.WithExpiration(*expiration))
				}

				claim, err := assert.Location.Invoke(
					principal,
					principal.DID(),
					&newArgs,
					opts...,
				)
				if err != nil {
					return nil, fmt.Errorf("delegating compressed location claim: %w", err)
				}

				newClaims := map[cid.Cid]ucan.Invocation{claim.Link(): claim}
				return Build(newClaims, bytemap.NewByteMap[types.EncodedContextID, blobindex.ShardedDagIndex](-1))
			}
		}
	}
	// never found the MH in any index shard, just build the regular query result
	return Build(claims, indexes)
}
