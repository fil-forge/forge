package redis

import (
	"bytes"

	"github.com/fil-forge/forge/indexing-service/pkg/types"
	"github.com/fil-forge/forge/protocol/blobindex"
)

var (
	_ types.ShardedDagIndexStore = (*ShardedDagIndexStore)(nil)
)

// ShardedDagIndexStore is a RedisStore for storing sharded dag indexes that implements types.ShardedDagIndexStore
type ShardedDagIndexStore = Store[types.EncodedContextID, blobindex.ShardedDagIndex]

// NewShardedDagIndexStore returns a new instance of a ShardedDagIndex store using the given redis client
func NewShardedDagIndexStore(client Client, opts ...Option) *ShardedDagIndexStore {
	return NewStore(shardedDagIndexFromRedis, shardedDagIndexToRedis, encodedContextIDKeyString, client, opts...)
}

func shardedDagIndexFromRedis(data string) (blobindex.ShardedDagIndex, error) {
	return blobindex.Extract(bytes.NewReader([]byte(data)))
}

func shardedDagIndexToRedis(shardedDagIndex blobindex.ShardedDagIndex) (string, error) {
	var buf bytes.Buffer
	if err := blobindex.Archive(shardedDagIndex, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func encodedContextIDKeyString(encodedContextID types.EncodedContextID) string {
	return string(encodedContextID)
}
