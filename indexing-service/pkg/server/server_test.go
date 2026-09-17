package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fil-forge/forge/indexing-service/pkg/internal/testutil"
	"github.com/fil-forge/forge/indexing-service/pkg/service/contentclaims"
	"github.com/fil-forge/ucantone/did"
	"github.com/fil-forge/ucantone/multikey"
	"github.com/ipfs/go-datastore"
	"github.com/stretchr/testify/require"
)

// TODO: TestGetClaimsHandler, TestGetIPNICIDHandler, TestGetDIDDocumentHandler,
// and the query/post-claim handler suites still need to be ported to the
// ucantone invocation flow. The legacy go-ucanto-based versions have been
// dropped during the migration.

func TestGetRootHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svr := httptest.NewServer(GetRootHandler(testutil.Service))
		defer svr.Close()

		res, err := http.Get(svr.URL)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, res.StatusCode)

		bytes, err := io.ReadAll(res.Body)
		require.NoError(t, err)

		require.True(t, strings.Contains(string(bytes), testutil.Service.DID().String()))
	})

	t.Run("did:web", func(t *testing.T) {
		didweb, err := did.Parse("did:web:example.org")
		require.NoError(t, err)

		s := multikey.NewIssuer(didweb, testutil.Service)

		svr := httptest.NewServer(GetRootHandler(s))
		defer svr.Close()

		res, err := http.Get(svr.URL)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, res.StatusCode)

		bytes, err := io.ReadAll(res.Body)
		require.NoError(t, err)

		body := string(bytes)
		require.Contains(t, body, testutil.Service.Verifier().String())
		require.Contains(t, body, didweb.String())
	})
}

func TestGetClaimHandler(t *testing.T) {
	store := contentclaims.NewStoreFromDatastore(datastore.NewMapDatastore())
	claim := testutil.RandomIndexClaim(t)
	err := store.Put(context.Background(), claim.Link(), claim)
	require.NoError(t, err)

	svr := httptest.NewServer(GetClaimHandler(store))
	defer svr.Close()

	t.Run("success", func(t *testing.T) {
		res, err := http.Get(fmt.Sprintf("%s/claim/%s", svr.URL, claim.Link()))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, res.StatusCode)

		bytes, err := io.ReadAll(res.Body)
		require.NoError(t, err)

		require.Equal(t, claim.Bytes(), bytes)
	})

	t.Run("not found", func(t *testing.T) {
		res, err := http.Get(fmt.Sprintf("%s/claim/%s", svr.URL, testutil.RandomCID(t)))
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, res.StatusCode)
	})

	t.Run("invalid CID", func(t *testing.T) {
		res, err := http.Get(fmt.Sprintf("%s/claim/invalid", svr.URL))
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, res.StatusCode)
	})
}
