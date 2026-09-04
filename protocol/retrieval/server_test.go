package retrieval_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/execution"
	"github.com/fil-forge/ucantone/ipld/datamodel"
	"github.com/fil-forge/ucantone/testutil"
	"github.com/fil-forge/ucantone/ucan/command"
	"github.com/fil-forge/ucantone/ucan/container"
	"github.com/fil-forge/ucantone/ucan/invocation"
	"github.com/stretchr/testify/require"

	"github.com/fil-forge/forge/protocol/retrieval"
)

var contentRetrieve = binding.Bind[*datamodel.Map, *datamodel.Map](command.MustParse("/content/retrieve"))

func TestServer(t *testing.T) {
	service := testutil.RandomIssuer(t)
	alice := testutil.RandomIssuer(t)

	blobBytes := []byte("retrieved blob bytes")
	const customHeader = "X-Test-Custom"
	const customHeaderValue = "hello-world"

	var (
		capturedMethod string
		capturedURL    *url.URL
		capturedHeader http.Header
	)

	s := retrieval.NewServer(service)
	s.Handle(contentRetrieve.Command, func(req execution.Request, res execution.Response) error {
		hcReq, ok := req.Metadata().(*retrieval.HTTPHeaderRequestContainer)
		require.True(t, ok, "expected HTTPHeaderRequestContainer as request metadata")
		capturedMethod = hcReq.Method
		capturedURL = hcReq.URL
		capturedHeader = hcReq.Header

		respMeta := &retrieval.HTTPHeaderResponseContainer{
			Container:  container.New(),
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"application/octet-stream"},
			},
			Body: io.NopCloser(bytes.NewReader(blobBytes)),
		}
		if err := res.SetMetadata(respMeta); err != nil {
			return err
		}
		return res.SetSuccess(datamodel.Map{})
	})

	httpServer := httptest.NewServer(s)
	t.Cleanup(httpServer.Close)

	inv, err := contentRetrieve.Invoke(
		alice,
		alice.DID(),
		&datamodel.Map{},
		invocation.WithAudience(service.DID()),
	)
	require.NoError(t, err)

	reqContainer := container.New(container.WithInvocations(inv))
	ctBytes, err := container.Encode(container.Base64Gzip, reqContainer)
	require.NoError(t, err)

	httpReq, err := http.NewRequest(http.MethodGet, httpServer.URL+"/blob/abc123", nil)
	require.NoError(t, err)
	httpReq.Header.Set(retrieval.HTTPHeaderName, string(ctBytes))
	httpReq.Header.Set(customHeader, customHeaderValue)

	httpResp, err := http.DefaultClient.Do(httpReq)
	require.NoError(t, err)
	t.Cleanup(func() { httpResp.Body.Close() })

	body, err := io.ReadAll(httpResp.Body)
	require.NoError(t, err)

	// The response body should carry the retrieved blob bytes.
	require.Equal(t, http.StatusOK, httpResp.StatusCode)
	require.Equal(t, "application/octet-stream", httpResp.Header.Get("Content-Type"))
	require.Equal(t, blobBytes, body)

	// The handler should have observed the HTTP request properties.
	require.Equal(t, http.MethodGet, capturedMethod)
	require.NotNil(t, capturedURL)
	require.Equal(t, "/blob/abc123", capturedURL.Path)
	require.Equal(t, customHeaderValue, capturedHeader.Get(customHeader))
	require.Equal(t, string(ctBytes), capturedHeader.Get(retrieval.HTTPHeaderName))

	// The UCAN response container travels in the X-UCAN-Container header.
	respHeaderCT := httpResp.Header.Get(retrieval.HTTPHeaderName)
	require.NotEmpty(t, respHeaderCT)
	require.Contains(t, httpResp.Header.Values("Vary"), retrieval.HTTPHeaderName)

	respContainer, err := container.Decode([]byte(respHeaderCT))
	require.NoError(t, err)
	require.Len(t, respContainer.Receipts(), 1)
	_, x := respContainer.Receipts()[0].Out().Unpack()
	require.Nil(t, x)
}
