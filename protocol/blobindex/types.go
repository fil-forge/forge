package blobindex

import (
	dm "github.com/fil-forge/forge/protocol/blobindex/datamodel"
	"github.com/fil-forge/forge/protocol/bytemap"
	mh "github.com/multiformats/go-multihash"
)

// MultihashMap is a map for mapping multihash digests to arbitrary data types.
type MultihashMap[T any] interface {
	bytemap.ByteMap[mh.Multihash, T]
}

// Range describes a start and end byte offset within a shard (inclusive).
type Range = dm.RangeModel

// ShardedDagIndex is a blob index for a DAG stored over one or more shards.
type ShardedDagIndex interface {
	// Index information for shards the DAG is split across.
	Shards() MultihashMap[MultihashMap[Range]]
}
