package blobindexlookup_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/fil-forge/indexing-service/pkg/internal/testutil"
	"github.com/fil-forge/indexing-service/pkg/service/blobindexlookup"
	"github.com/fil-forge/indexing-service/pkg/types"
	"github.com/fil-forge/libforge/blobindex"
	"github.com/fil-forge/libforge/commands/content"
	"github.com/fil-forge/libforge/ucan/retrieval"
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/execution"
	"github.com/fil-forge/ucantone/ipld/datamodel"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/fil-forge/ucantone/ucan/command"
	"github.com/fil-forge/ucantone/ucan/container"
	"github.com/stretchr/testify/require"
)

var contentRetrieve = binding.Bind[*datamodel.Map, *datamodel.Map](command.MustParse("/content/retrieve"))

func TestBlobIndexLookup__Find(t *testing.T) {
	provider := testutil.RandomProviderResult(t)
	_, index := testutil.RandomShardedDagIndex(t, 32)

	var indexBuf bytes.Buffer
	require.NoError(t, blobindex.Archive(index, &indexBuf))
	indexBytes := indexBuf.Bytes()
	indexEncodedLength := uint64(len(indexBytes))

	srv := retrieval.NewServer(testutil.Service)
	srv.Handle(contentRetrieve.Command, func(req execution.Request, res execution.Response) error {
		respMeta := &retrieval.HTTPHeaderResponseContainer{
			Container:  container.New(),
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"application/octet-stream"},
			},
			Body: io.NopCloser(bytes.NewReader(indexBytes)),
		}
		if err := res.SetMetadata(respMeta); err != nil {
			return err
		}
		return res.SetSuccess(&datamodel.Map{})
	})

	testServer := httptest.NewServer(srv)
	t.Cleanup(testServer.Close)

	contentDigest := testutil.RandomMultihash(t)

	req := types.RetrievalRequest{
		URL: *testutil.Must(url.Parse(testServer.URL))(t),
		Range: &content.Range{
			Start: 0,
			End:   indexEncodedLength - 1,
		},
		Auth: types.RetrievalAuth{
			Issuer:   testutil.Service,
			Audience: testutil.Service.DID(),
			Command:  content.Retrieve.Command,
			Subject:  testutil.Service.DID(),
			Arguments: &content.RetrieveArguments{
				Blob:  content.Blob{Digest: contentDigest},
				Range: content.Range{Start: 0, End: indexEncodedLength - 1},
			},
			Proofs: []ucan.Delegation{},
		},
	}

	cl := blobindexlookup.NewBlobIndexLookup(testServer.Client())
	got, err := cl.Find(context.Background(), types.EncodedContextID(contentDigest), provider, req)
	require.NoError(t, err)
	testutil.RequireEqualIndex(t, index, got)
}
