package providercacher

import (
	"context"
	"fmt"

	"github.com/fil-forge/forge/indexing-service/pkg/telemetry"
	"github.com/fil-forge/forge/indexing-service/pkg/types"
	"github.com/fil-forge/forge/protocol/blobindex"
	"github.com/ipni/go-libipni/find/model"
	"go.opentelemetry.io/otel/attribute"
)

// MaxBatchSize is the maximum number of items that'll be added to a batch.
const MaxBatchSize = 10_000

type simpleProviderCacher struct {
	providerStore types.ProviderStore
}

func NewSimpleProviderCacher(providerStore types.ProviderStore) ProviderCacher {
	return &simpleProviderCacher{providerStore: providerStore}
}

func (s *simpleProviderCacher) CacheProviderForIndexRecords(ctx context.Context, provider model.ProviderResult, index blobindex.ShardedDagIndex) error {
	ctx, span := telemetry.StartSpan(ctx, "ProviderCacher.CacheProviderForIndexRecords")
	defer span.End()

	batch := s.providerStore.Batch()
	total := 0
	size := 1
	for _, shardIndex := range index.Shards().Iterator() {
		for hash := range shardIndex.Iterator() {
			err := batch.Add(ctx, hash, provider)
			if err != nil {
				return fmt.Errorf("batch adding provider: %w", err)
			}
			err = batch.SetExpirable(ctx, hash, true)
			if err != nil {
				return fmt.Errorf("batch setting provider expirable: %w", err)
			}
			total++
			size++
			if size >= MaxBatchSize {
				span.AddEvent("commit batch")
				err := batch.Commit(ctx)
				if err != nil {
					return fmt.Errorf("batch commiting: %w", err)
				}
				batch = s.providerStore.Batch()
				size = 0
			}
		}
	}
	span.SetAttributes(attribute.KeyValue{Key: "total", Value: attribute.IntValue(total)})
	if size == 0 {
		return nil
	}
	span.AddEvent("commit batch")
	return batch.Commit(ctx)
}
