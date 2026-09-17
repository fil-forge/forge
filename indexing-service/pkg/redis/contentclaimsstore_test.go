package redis_test

import (
	"context"
	"testing"

	"github.com/fil-forge/indexing-service/pkg/internal/testutil"
	"github.com/fil-forge/indexing-service/pkg/redis"
	"github.com/fil-forge/libforge/commands"
	assertcaps "github.com/fil-forge/libforge/commands/assert"
	"github.com/stretchr/testify/require"
)

func TestContentClaimsStore(t *testing.T) {
	mockRedis := NewMockRedis()
	contentClaimsStore := redis.NewContentClaimsStore(mockRedis)

	claim1, err := assertcaps.Location.Invoke(
		testutil.Service,
		testutil.Service.DID(),
		&assertcaps.LocationArguments{
			Space:    testutil.Service.DID(),
			Content:  testutil.RandomMultihash(t),
			Location: []commands.CborURL{commands.CborURL(*testutil.TestURL)},
		},
	)
	require.NoError(t, err)

	claim2, err := assertcaps.Index.Invoke(
		testutil.Service,
		testutil.Service.DID(),
		&assertcaps.IndexArguments{Index: testutil.RandomCID(t)},
	)
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, contentClaimsStore.Set(ctx, claim1.Link(), claim1, false))
	require.NoError(t, contentClaimsStore.Set(ctx, claim2.Link(), claim2, true))

	returnedClaim1 := testutil.Must(contentClaimsStore.Get(ctx, claim1.Link()))(t)
	returnedClaim2 := testutil.Must(contentClaimsStore.Get(ctx, claim2.Link()))(t)
	testutil.RequireEqualClaim(t, claim1, returnedClaim1)
	testutil.RequireEqualClaim(t, claim2, returnedClaim2)
}
