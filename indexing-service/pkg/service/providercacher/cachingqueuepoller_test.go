package providercacher_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/fil-forge/go-ipni-tools/pkg/queue"
	"github.com/fil-forge/indexing-service/pkg/service/providercacher"
	"github.com/fil-forge/libforge/blobindex"
	"github.com/ipni/go-libipni/find/model"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCachingQueuePoller_StartStop(t *testing.T) {
	mockQueue := providercacher.NewMockCachingQueue(t)
	mockCacher := providercacher.NewMockProviderCacher(t)

	mockQueue.EXPECT().Read(mock.Anything, mock.Anything).Return([]queue.WithID[providercacher.ProviderCachingJob]{}, nil)

	poller, err := providercacher.NewCachingQueuePoller(mockQueue, mockCacher)
	require.NoError(t, err)

	poller.Start()
	time.Sleep(20 * time.Millisecond)
	poller.Stop()
}

func TestCachingQueuePoller_BatchProcessing(t *testing.T) {
	const (
		numJobs       = 11
		batchSize     = 2
		fullBatches   = numJobs / batchSize
		lastBatchSize = numJobs % batchSize
	)

	testJob := queue.WithID[providercacher.ProviderCachingJob]{
		ID: "test-job",
		Job: providercacher.ProviderCachingJob{
			Provider: model.ProviderResult{Provider: &peer.AddrInfo{ID: peer.ID("test-peer")}},
			Index:    blobindex.NewShardedDagIndex(0),
		},
	}

	mockQueue := providercacher.NewMockCachingQueue(t)
	mockCacher := providercacher.NewMockProviderCacher(t)

	batch := make([]queue.WithID[providercacher.ProviderCachingJob], batchSize)
	for i := range batchSize {
		batch[i] = testJob
	}
	mockQueue.EXPECT().Read(mock.Anything, batchSize).Return(batch, nil).Times(fullBatches)
	mockQueue.EXPECT().Read(mock.Anything, batchSize).Return(batch[:lastBatchSize], nil).Once()
	mockQueue.EXPECT().Read(mock.Anything, batchSize).Return([]queue.WithID[providercacher.ProviderCachingJob]{}, nil).
		Run(func(ctx context.Context, _ int) {
			<-ctx.Done()
		}).
		Return([]queue.WithID[providercacher.ProviderCachingJob]{}, nil).
		Once()

	var wg sync.WaitGroup
	wg.Add(numJobs)

	mockCacher.EXPECT().
		CacheProviderForIndexRecords(mock.Anything, testJob.Job.Provider, testJob.Job.Index).
		Run(func(ctx context.Context, _ model.ProviderResult, _ blobindex.ShardedDagIndex) {
			defer wg.Done()
		}).
		Return(nil).
		Times(numJobs)

	mockQueue.EXPECT().
		Delete(mock.Anything, testJob.ID).
		Return(nil).
		Times(numJobs)

	poller, err := providercacher.NewCachingQueuePoller(
		mockQueue,
		mockCacher,
		queue.WithJobBatchSize(batchSize),
	)
	require.NoError(t, err)

	poller.Start()
	wg.Wait()
	poller.Stop()
}

func TestCachingQueuePoller_FailedJobsAreRetried(t *testing.T) {
	successfulJob := queue.WithID[providercacher.ProviderCachingJob]{
		ID: "successful-job",
		Job: providercacher.ProviderCachingJob{
			Provider: model.ProviderResult{Provider: &peer.AddrInfo{ID: peer.ID("successful-peer")}},
			Index:    blobindex.NewShardedDagIndex(0),
		},
	}

	failedJob := queue.WithID[providercacher.ProviderCachingJob]{
		ID: "failed-job",
		Job: providercacher.ProviderCachingJob{
			Provider: model.ProviderResult{Provider: &peer.AddrInfo{ID: peer.ID("failed-peer")}},
			Index:    blobindex.NewShardedDagIndex(0),
		},
	}

	mockQueue := providercacher.NewMockCachingQueue(t)
	mockCacher := providercacher.NewMockProviderCacher(t)

	mockQueue.EXPECT().Read(mock.Anything, mock.Anything).Return(
		[]queue.WithID[providercacher.ProviderCachingJob]{successfulJob, failedJob}, nil,
	).Once()
	mockQueue.EXPECT().Read(mock.Anything, mock.Anything).Return([]queue.WithID[providercacher.ProviderCachingJob]{}, nil).
		Run(func(ctx context.Context, _ int) {
			<-ctx.Done()
		}).
		Return([]queue.WithID[providercacher.ProviderCachingJob]{}, nil).
		Once()

	mockCacher.EXPECT().CacheProviderForIndexRecords(mock.Anything, successfulJob.Job.Provider, successfulJob.Job.Index).
		Return(nil).Once()
	mockCacher.EXPECT().CacheProviderForIndexRecords(mock.Anything, failedJob.Job.Provider, failedJob.Job.Index).
		Return(errors.New("processing error")).Once()

	mockQueue.EXPECT().Delete(mock.Anything, successfulJob.ID).Return(nil).Once()
	mockQueue.EXPECT().Release(mock.Anything, failedJob.ID).Return(nil).Once()

	poller, err := providercacher.NewCachingQueuePoller(mockQueue, mockCacher)
	require.NoError(t, err)

	poller.Start()
	time.Sleep(50 * time.Millisecond)
	poller.Stop()
}

func TestCachingQueuePoller_JobsTimingOutAreNotRetried(t *testing.T) {
	bigJob := queue.WithID[providercacher.ProviderCachingJob]{
		ID: "big-job",
		Job: providercacher.ProviderCachingJob{
			Provider: model.ProviderResult{Provider: &peer.AddrInfo{ID: peer.ID("peer")}},
			Index:    blobindex.NewShardedDagIndex(0),
		},
	}

	mockQueue := providercacher.NewMockCachingQueue(t)
	mockCacher := providercacher.NewMockProviderCacher(t)

	mockQueue.EXPECT().Read(mock.Anything, mock.Anything).Return(
		[]queue.WithID[providercacher.ProviderCachingJob]{bigJob}, nil,
	).Once()
	mockQueue.EXPECT().Read(mock.Anything, mock.Anything).Return([]queue.WithID[providercacher.ProviderCachingJob]{}, nil).
		Run(func(ctx context.Context, _ int) {
			<-ctx.Done()
		}).
		Return([]queue.WithID[providercacher.ProviderCachingJob]{}, nil).
		Once()

	mockCacher.EXPECT().CacheProviderForIndexRecords(mock.Anything, bigJob.Job.Provider, bigJob.Job.Index).
		Return(context.DeadlineExceeded).Once()

	mockQueue.EXPECT().Delete(mock.Anything, bigJob.ID).Return(nil).Once()

	poller, err := providercacher.NewCachingQueuePoller(mockQueue, mockCacher)
	require.NoError(t, err)

	poller.Start()
	time.Sleep(50 * time.Millisecond)
	poller.Stop()
}
