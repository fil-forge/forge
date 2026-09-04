package retrieval_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/fil-forge/ucantone/execution"
	"github.com/fil-forge/ucantone/ipld/datamodel"
	"github.com/fil-forge/ucantone/testutil"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/fil-forge/ucantone/ucan/container"
	"github.com/fil-forge/ucantone/ucan/invocation"
	"github.com/stretchr/testify/require"

	"github.com/fil-forge/forge/protocol/retrieval"
)

// startTestServer spins up a retrieval server that registers the given
// handler for `/content/retrieve` and returns its base URL plus the service
// issuer.
func startTestServer(t *testing.T, handler execution.HandlerFunc) (*url.URL, ucan.Issuer) {
	t.Helper()
	service := testutil.RandomIssuer(t)
	s := retrieval.NewServer(service)
	s.Handle(contentRetrieve.Command, handler)
	httpServer := httptest.NewServer(s)
	t.Cleanup(httpServer.Close)
	u, err := url.Parse(httpServer.URL)
	require.NoError(t, err)
	return u, service
}

func TestClient(t *testing.T) {
	t.Run("execute round trip", func(t *testing.T) {
		alice := testutil.RandomIssuer(t)
		blobBytes := []byte("retrieved blob bytes")

		serviceURL, service := startTestServer(t, func(req execution.Request, res execution.Response) error {
			require.NoError(t, res.SetMetadata(&retrieval.HTTPHeaderResponseContainer{
				Container:  container.New(),
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"application/octet-stream"},
				},
				Body: io.NopCloser(bytes.NewReader(blobBytes)),
			}))
			return res.SetSuccess(datamodel.Map{})
		})

		client, err := retrieval.NewClient(serviceURL)
		require.NoError(t, err)

		inv, err := contentRetrieve.Invoke(
			alice,
			alice.DID(),
			&datamodel.Map{},
			invocation.WithAudience(service.DID()),
		)
		require.NoError(t, err)

		res, err := client.Execute(execution.NewRequest(context.Background(), inv))
		require.NoError(t, err)

		// The receipt for the executed task should be a success.
		require.NotNil(t, res.Receipt())
		require.Equal(t, inv.Task().Link(), res.Receipt().Ran())
		_, x := res.Receipt().Out().Unpack()
		require.Nil(t, x)

		// The response metadata is a *HTTPHeaderResponseContainer carrying the
		// retrieved blob bytes — the caller is responsible for closing the body.
		hcRes, ok := res.Metadata().(*retrieval.HTTPHeaderResponseContainer)
		require.True(t, ok, "expected response metadata to be a *HTTPHeaderResponseContainer")
		require.Equal(t, http.StatusOK, hcRes.StatusCode)
		require.Equal(t, "application/octet-stream", hcRes.Header.Get("Content-Type"))

		body, err := io.ReadAll(hcRes.Body)
		require.NoError(t, err)
		require.NoError(t, hcRes.Body.Close())
		require.Equal(t, blobBytes, body)
	})

	t.Run("with HTTP headers adds headers to every request", func(t *testing.T) {
		alice := testutil.RandomIssuer(t)
		const headerName = "X-Test-Auth"
		const headerValue = "token-123"

		var seen []string
		serviceURL, service := startTestServer(t, func(req execution.Request, res execution.Response) error {
			hcReq, ok := req.Metadata().(*retrieval.HTTPHeaderRequestContainer)
			require.True(t, ok)
			seen = append(seen, hcReq.Header.Get(headerName))
			return res.SetSuccess(datamodel.Map{})
		})

		client, err := retrieval.NewClient(serviceURL, retrieval.WithHTTPHeaders(http.Header{
			headerName: []string{headerValue},
		}))
		require.NoError(t, err)

		for range 2 {
			inv, err := contentRetrieve.Invoke(
				alice,
				alice.DID(),
				&datamodel.Map{},
				invocation.WithAudience(service.DID()),
			)
			require.NoError(t, err)
			_, err = client.Execute(execution.NewRequest(context.Background(), inv))
			require.NoError(t, err)
		}

		require.Equal(t, []string{headerValue, headerValue}, seen)
	})

	t.Run("with event listener observes request and response", func(t *testing.T) {
		alice := testutil.RandomIssuer(t)

		serviceURL, service := startTestServer(t, func(req execution.Request, res execution.Response) error {
			return res.SetSuccess(datamodel.Map{})
		})

		listener := &recordingListener{}
		client, err := retrieval.NewClient(serviceURL, retrieval.WithEventListener(listener))
		require.NoError(t, err)

		inv, err := contentRetrieve.Invoke(
			alice,
			alice.DID(),
			&datamodel.Map{},
			invocation.WithAudience(service.DID()),
		)
		require.NoError(t, err)

		_, err = client.Execute(execution.NewRequest(context.Background(), inv))
		require.NoError(t, err)

		require.NotNil(t, listener.encoded, "OnRequestEncode should have been called")
		require.Len(t, listener.encoded.Invocations(), 1)
		require.Equal(t, inv.Task().Link(), listener.encoded.Invocations()[0].Task().Link())

		require.NotNil(t, listener.decoded, "OnResponseDecode should have been called")
		require.Len(t, listener.decoded.Receipts(), 1)
		require.Equal(t, inv.Task().Link(), listener.decoded.Receipts()[0].Ran())
	})

	t.Run("with HTTP client uses provided client", func(t *testing.T) {
		alice := testutil.RandomIssuer(t)

		serviceURL, service := startTestServer(t, func(req execution.Request, res execution.Response) error {
			return res.SetSuccess(datamodel.Map{})
		})

		httpClient := &http.Client{Transport: &countingTransport{inner: http.DefaultTransport}}
		client, err := retrieval.NewClient(serviceURL, retrieval.WithHTTPClient(httpClient))
		require.NoError(t, err)

		inv, err := contentRetrieve.Invoke(
			alice,
			alice.DID(),
			&datamodel.Map{},
			invocation.WithAudience(service.DID()),
		)
		require.NoError(t, err)

		_, err = client.Execute(execution.NewRequest(context.Background(), inv))
		require.NoError(t, err)

		require.Equal(t, 1, httpClient.Transport.(*countingTransport).count)
	})
}

type recordingListener struct {
	encoded ucan.Container
	decoded ucan.Container
}

func (l *recordingListener) OnRequestEncode(_ context.Context, ct ucan.Container) error {
	l.encoded = ct
	return nil
}

func (l *recordingListener) OnResponseDecode(_ context.Context, ct ucan.Container) error {
	l.decoded = ct
	return nil
}

type countingTransport struct {
	inner http.RoundTripper
	count int
}

func (t *countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.count++
	return t.inner.RoundTrip(r)
}
