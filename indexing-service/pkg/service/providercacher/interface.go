package providercacher

import (
	"context"

	"github.com/fil-forge/forge/protocol/blobindex"
	"github.com/ipni/go-libipni/find/model"
)

type ProviderCacher interface {
	CacheProviderForIndexRecords(ctx context.Context, provider model.ProviderResult, index blobindex.ShardedDagIndex) error
}
