package client

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/fil-forge/automobile"
	"github.com/fil-forge/forge/indexing-service/pkg/internal/testutil"
	"github.com/fil-forge/forge/indexing-service/pkg/service/contentclaims"
	"github.com/fil-forge/forge/indexing-service/pkg/service/queryresult"
	"github.com/fil-forge/forge/indexing-service/pkg/types"
	"github.com/fil-forge/forge/protocol/blobindex"
	"github.com/fil-forge/forge/protocol/bytemap"
	"github.com/fil-forge/forge/protocol/commands"
	assertcaps "github.com/fil-forge/forge/protocol/commands/assert"
	claimcaps "github.com/fil-forge/forge/protocol/commands/claim"
	"github.com/fil-forge/forge/protocol/digestutil"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/ipfs/go-cid"
	"github.com/ipni/go-libipni/maurl"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	indexingID := testutil.Service

	// Track invocations seen by the indexing UCAN server.
	indexer := newRecordingIndexer()
	ucanSrv, err := contentclaims.NewUCANServer(indexingID, indexer)
	require.NoError(t, err)
	indexingUCANServer := httptest.NewServer(ucanSrv)
	t.Cleanup(indexingUCANServer.Close)
	indexingURL := *testutil.Must(url.Parse(indexingUCANServer.URL))(t)

	// Self-signed invocations: use the indexing service signer as the issuer
	// so the server-side validator accepts them without a delegation chain.
	storageID := indexingID
	storageURL := indexingURL
	alice := indexingID

	contentDigest := testutil.RandomMultihash(t)
	indexCID := testutil.RandomCID(t)

	locationClaim, err := assertcaps.Location.Invoke(
		storageID,
		storageID.DID(),
		&assertcaps.LocationArguments{
			Space:    alice.DID(),
			Content:  contentDigest,
			Location: []commands.CborURL{commands.CborURL(*testutil.TestURL)},
		},
	)
	require.NoError(t, err)

	provider := claimcaps.Provider{
		Addresses: [][]byte{
			testutil.Must(maurl.FromURL(storageURL.JoinPath("/claim/{claim}")))(t).Bytes(),
			testutil.Must(maurl.FromURL(storageURL.JoinPath("/blob/{blob}")))(t).Bytes(),
		},
	}

	t.Run("cache claim", func(t *testing.T) {
		indexer.reset()
		c, err := New(indexingID.DID(), indexingURL)
		require.NoError(t, err)

		err = c.CacheClaim(context.Background(), storageID, locationClaim, provider)
		require.NoError(t, err)

		require.NotEmpty(t, indexer.cached)
		require.Equal(t, locationClaim.Link(), indexer.cached[len(indexer.cached)-1].Link())
	})

	t.Run("publish index claim", func(t *testing.T) {
		indexer.reset()
		c, err := New(indexingID.DID(), indexingURL)
		require.NoError(t, err)

		err = c.PublishIndexClaim(
			context.Background(),
			alice,
			assertcaps.IndexArguments{Index: indexCID},
		)
		require.NoError(t, err)

		require.NotEmpty(t, indexer.published)
		require.Equal(t, assertcaps.Index.Command, indexer.published[len(indexer.published)-1].Command())
	})

	t.Run("query claims", func(t *testing.T) {
		for _, tc := range []struct {
			name       string
			detectGzip bool
		}{
			{name: "without gzip", detectGzip: false},
			{name: "with gzip", detectGzip: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Run("basic", func(t *testing.T) {
					results := bytemap.NewByteMap[multihash.Multihash, types.QueryResult](-1)
					srv := mockQueryServer(t, results, config{detectGzip: tc.detectGzip})
					t.Cleanup(srv.Close)

					c, err := New(indexingID.DID(), *testutil.Must(url.Parse(srv.URL))(t))
					require.NoError(t, err)

					claims := map[cid.Cid]ucan.Invocation{
						locationClaim.Link(): locationClaim,
					}
					indexes := bytemap.NewByteMap[types.EncodedContextID, blobindex.ShardedDagIndex](-1)
					results.Set(contentDigest, testutil.Must(queryresult.Build(claims, indexes))(t))

					res, err := c.QueryClaims(context.Background(), types.Query{
						Hashes: []multihash.Multihash{contentDigest},
					})
					require.NoError(t, err)

					require.True(t, slices.ContainsFunc(res.Claims(), func(c cid.Cid) bool {
						return c == locationClaim.Link()
					}))
				})

				t.Run("query requests the right type", func(t *testing.T) {
					results := bytemap.NewByteMap[multihash.Multihash, types.QueryResult](-1)
					srv := mockQueryServer(t, results, config{detectGzip: tc.detectGzip})
					t.Cleanup(srv.Close)

					c, err := New(indexingID.DID(), *testutil.Must(url.Parse(srv.URL))(t))
					require.NoError(t, err)

					claims := map[cid.Cid]ucan.Invocation{}
					indexes := bytemap.NewByteMap[types.EncodedContextID, blobindex.ShardedDagIndex](-1)
					results.Set(contentDigest, testutil.Must(queryresult.Build(claims, indexes))(t))

					t.Run("standard", func(t *testing.T) {
						_, err := c.QueryClaims(context.Background(), types.Query{Hashes: []multihash.Multihash{contentDigest}})
						require.NoError(t, err)
						require.Contains(t, srv.lastRequestedURL, "type=standard")
					})

					t.Run("location", func(t *testing.T) {
						_, err := c.QueryClaims(context.Background(), types.Query{
							Type:   types.QueryTypeLocation,
							Hashes: []multihash.Multihash{contentDigest},
						})
						require.NoError(t, err)
						require.Contains(t, srv.lastRequestedURL, "type=location")
					})

					t.Run("index_or_location", func(t *testing.T) {
						_, err := c.QueryClaims(context.Background(), types.Query{
							Type:   types.QueryTypeIndexOrLocation,
							Hashes: []multihash.Multihash{contentDigest},
						})
						require.NoError(t, err)
						require.Contains(t, srv.lastRequestedURL, "type=index_or_location")
					})
				})

				t.Run("query throws error", func(t *testing.T) {
					results := bytemap.NewByteMap[multihash.Multihash, types.QueryResult](-1)
					srv := mockQueryServer(t, results, config{detectGzip: tc.detectGzip, throwError: errors.New("something went terribly wrong")})
					t.Cleanup(srv.Close)

					c, err := New(indexingID.DID(), *testutil.Must(url.Parse(srv.URL))(t))
					require.NoError(t, err)

					_, err = c.QueryClaims(context.Background(), types.Query{
						Hashes: []multihash.Multihash{contentDigest},
					})
					require.Error(t, err)
					require.Contains(t, err.Error(), "something went terribly wrong")
				})
			})
		}
	})
}

// recordingIndexer captures invocations and acts as a UCAN service.
type recordingIndexer struct {
	cached    []ucan.Invocation
	published []ucan.Invocation
}

func newRecordingIndexer() *recordingIndexer { return &recordingIndexer{} }

func (r *recordingIndexer) reset() {
	r.cached = nil
	r.published = nil
}

func (r *recordingIndexer) Cache(ctx context.Context, provider peer.AddrInfo, claim ucan.Invocation, meta ucan.Container) error {
	r.cached = append(r.cached, claim)
	return nil
}

func (r *recordingIndexer) Publish(ctx context.Context, claim ucan.Invocation, meta ucan.Container) error {
	r.published = append(r.published, claim)
	return nil
}

type mockServer struct {
	*httptest.Server
	lastRequestedURL    string
	lastRequestedHeader http.Header
}

type config struct {
	throwError error
	detectGzip bool
}

func mockQueryServer(_ *testing.T, results bytemap.ByteMap[multihash.Multihash, types.QueryResult], cfg config) *mockServer {
	ms := &mockServer{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ms.lastRequestedURL = r.URL.String()
		ms.lastRequestedHeader = r.Header

		mhStrings := r.URL.Query()["multihash"]
		if len(mhStrings) != 1 {
			http.Error(w, "mock query service supports only single hash", http.StatusNotImplemented)
			return
		}

		digest, err := digestutil.Parse(mhStrings[0])
		if err != nil {
			http.Error(w, "invalid digest", http.StatusBadRequest)
			return
		}

		if cfg.throwError != nil {
			http.Error(w, cfg.throwError.Error(), http.StatusInternalServerError)
			return
		}

		qr := results.Get(digest)
		if qr == nil {
			qr, _ = queryresult.Build(
				map[cid.Cid]ucan.Invocation{},
				bytemap.NewByteMap[types.EncodedContextID, blobindex.ShardedDagIndex](-1),
			)
		}

		blks := make([]automobile.Block, 0, len(qr.Blocks()))
		for _, blk := range qr.Blocks() {
			blks = append(blks, automobile.Block{Link: blk.Link, Data: blk.Data})
		}
		w.WriteHeader(http.StatusOK)
		body := automobile.Encode([]cid.Cid{qr.Root().Link}, blks)
		_, _ = io.Copy(w, body)
	})

	if cfg.detectGzip {
		handler = withGzip(handler)
	}

	ms.Server = httptest.NewServer(handler)
	return ms
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipResponseWriter) Write(b []byte) (int, error) { return w.Writer.Write(b) }

func withGzip(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			handler(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		handler(gzipResponseWriter{Writer: gz, ResponseWriter: w}, r)
	}
}

// silence unused imports if some are not needed
var _ = multiaddr.NewMultiaddr
