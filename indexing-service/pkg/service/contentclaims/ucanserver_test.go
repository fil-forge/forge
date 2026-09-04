package contentclaims

import (
	"context"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/fil-forge/forge/indexing-service/pkg/internal/testutil"
	"github.com/fil-forge/forge/indexing-service/pkg/types"
	"github.com/fil-forge/forge/protocol/commands"
	assertcaps "github.com/fil-forge/forge/protocol/commands/assert"
	claimcaps "github.com/fil-forge/forge/protocol/commands/claim"
	"github.com/fil-forge/ucantone/client"
	"github.com/fil-forge/ucantone/execution"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	srv, err := NewUCANServer(testutil.Service, &mockIndexer{})
	require.NoError(t, err)

	httpSrv := httptest.NewServer(srv)
	t.Cleanup(httpSrv.Close)

	srvURL, err := url.Parse(httpSrv.URL)
	require.NoError(t, err)

	httpClient, err := client.NewHTTP(srvURL)
	require.NoError(t, err)

	// Build a self-signed location commitment to attach to the cache invocation.
	locationCommitment := testutil.Must(assertcaps.Location.Invoke(
		testutil.Alice,
		testutil.Alice.DID(),
		&assertcaps.LocationArguments{
			Space:    testutil.Bob.DID(),
			Content:  testutil.RandomMultihash(t),
			Location: []commands.CborURL{commands.CborURL(*testutil.TestURL)},
		},
	))(t)

	cacheInvocation := testutil.Must(claimcaps.Cache.Invoke(
		testutil.Service,
		testutil.Service.DID(),
		&claimcaps.CacheArguments{
			Claim: locationCommitment.Link(),
			Provider: claimcaps.Provider{
				Addresses: [][]byte{testutil.RandomMultiaddr(t).Bytes()},
			},
		},
	))(t)

	invs := []struct {
		name string
		inv  ucan.Invocation
		opts []execution.RequestOption
	}{
		{
			name: assertcaps.Equals.String(),
			inv: testutil.Must(assertcaps.Equals.Invoke(
				testutil.Service,
				testutil.Service.DID(),
				&assertcaps.EqualsArguments{
					Content: testutil.RandomMultihash(t),
					Equals:  testutil.RandomCID(t),
				},
			))(t),
		},
		{
			name: assertcaps.Index.String(),
			inv: testutil.Must(assertcaps.Index.Invoke(
				testutil.Service,
				testutil.Service.DID(),
				&assertcaps.IndexArguments{
					Index: testutil.RandomCID(t),
				},
			))(t),
		},
		{
			name: claimcaps.Cache.String(),
			inv:  cacheInvocation,
			opts: []execution.RequestOption{execution.WithInvocations(locationCommitment)},
		},
	}

	for _, tc := range invs {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := httpClient.Execute(execution.NewRequest(t.Context(), tc.inv, tc.opts...))
			require.NoError(t, err)
			require.False(t, resp.Receipt().Out().IsErr(), "unexpected failure")
		})
	}
}

type mockIndexer struct{}

func (m *mockIndexer) Get(ctx context.Context, claim cid.Cid) (ucan.Invocation, error) {
	return nil, nil
}

func (m *mockIndexer) Cache(ctx context.Context, provider peer.AddrInfo, claim ucan.Invocation, meta ucan.Container) error {
	return nil
}

func (m *mockIndexer) Publish(ctx context.Context, claim ucan.Invocation, meta ucan.Container) error {
	return nil
}

func (m *mockIndexer) Query(ctx context.Context, q types.Query) (types.QueryResult, error) {
	return nil, nil
}

var _ types.Service = (*mockIndexer)(nil)
