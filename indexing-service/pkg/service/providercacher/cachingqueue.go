package providercacher

import (
	"context"

	"github.com/fil-forge/go-ipni-tools/pkg/queue"
	"github.com/fil-forge/libforge/blobindex"
	"github.com/ipni/go-libipni/find/model"
)

type (
	CachingQueueQueuer = queue.QueueQueuer[ProviderCachingJob]
	CachingQueue       = queue.Queue[ProviderCachingJob]

	ProviderCachingJob struct {
		Provider model.ProviderResult
		Index    blobindex.ShardedDagIndex
	}

	JobHandler struct {
		providerCacher ProviderCacher
	}
)

func NewJobHandler(providerCacher ProviderCacher) *JobHandler {
	return &JobHandler{
		providerCacher: providerCacher,
	}
}

func (j *JobHandler) Handle(ctx context.Context, job ProviderCachingJob) error {
	return j.providerCacher.CacheProviderForIndexRecords(ctx, job.Provider, job.Index)
}
