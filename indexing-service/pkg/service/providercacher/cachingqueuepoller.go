package providercacher

import (
	"github.com/fil-forge/go-ipni-tools/pkg/queue"
)

// CachingQueuePoller polls a queue for provider caching jobs and processes them
// using the provided ProviderCacher and SQSCachingDecoder.
type CachingQueuePoller = queue.QueuePoller[ProviderCachingJob]

// NewCachingQueuePoller creates a new CachingQueuePoller instance.
func NewCachingQueuePoller(cachingQueue CachingQueue, cacher ProviderCacher, opts ...queue.Option) (*CachingQueuePoller, error) {
	return queue.NewQueuePoller(cachingQueue, queue.JobHandler(NewJobHandler(cacher).Handle), opts...)
}
