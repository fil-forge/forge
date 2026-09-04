package datamodel

import (
	cid "github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
)

// ShardedDagIndexModel is the golang structure for encoding sharded DAG index header blocks
type ShardedDagIndexModel struct {
	DagO_1 *ShardedDagIndexModel_0_1 `cborgen:"index/sharded/dag@0.1,omitempty" dagjsongen:"index/sharded/dag@0.1,omitempty"`
}

// ShardedDagIndexModel_0_1 describes the 0.1 version of ShardedDagIndex
type ShardedDagIndexModel_0_1 struct {
	Shards []cid.Cid `cborgen:"shards" dagjsongen:"shards"`
}

// RangeModel is a start and end byte offset for a slice in a blob (inclusive)
type RangeModel struct {
	Start int64
	End   int64
}

// BlobSliceModel describes a multihash and its byte offset in a blob
type BlobSliceModel struct {
	Digest multihash.Multihash
	Range  RangeModel
}

// BlobIndexModel is the golang structure for encoding a shard of CIDs in a block
type BlobIndexModel struct {
	Digest multihash.Multihash
	Slices []BlobSliceModel
}
