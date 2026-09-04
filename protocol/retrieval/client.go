package retrieval

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/fil-forge/ucantone/client"
	"github.com/fil-forge/ucantone/execution"
)

type HTTPHeaderClient struct {
	*client.Client[*http.Request, *http.Response]
}

type httpHeaderClientConfig struct {
	client    *http.Client
	headers   http.Header
	listeners []client.EventListener
}

type ClientOption func(*httpHeaderClientConfig) error

func WithHTTPClient(client *http.Client) ClientOption {
	return func(cfg *httpHeaderClientConfig) error {
		cfg.client = client
		return nil
	}
}

// WithHTTPHeaders adds custom HTTP headers to EVERY request. Note that the
// "X-UCAN-Container" header is reserved for encoding the UCAN invocation and
// should not be included in the provided headers.
func WithHTTPHeaders(headers http.Header) ClientOption {
	return func(cfg *httpHeaderClientConfig) error {
		if headers.Get(HTTPHeaderName) != "" {
			return fmt.Errorf("cannot set %q header with WithHTTPHeaders, it is reserved for encoding the UCAN invocation", HTTPHeaderName)
		}
		cfg.headers = headers
		return nil
	}
}

// WithEventListener adds an event listener to the HTTP client for monitoring
// requests and responses.
func WithEventListener(listener client.EventListener) ClientOption {
	return func(cfg *httpHeaderClientConfig) error {
		cfg.listeners = append(cfg.listeners, listener)
		return nil
	}
}

func NewClient(serviceURL *url.URL, options ...ClientOption) (*HTTPHeaderClient, error) {
	codec := DefaultHTTPHeaderOutboundCodec
	cfg := httpHeaderClientConfig{
		client: http.DefaultClient,
	}
	for _, opt := range options {
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}
	c := client.New(&httpTransport{cfg.client, cfg.headers, serviceURL}, codec)
	c.Listeners = cfg.listeners
	return &HTTPHeaderClient{Client: c}, nil
}

// Note: execution response metadata is a [HTTPHeaderResponseContainer] and it
// is the caller's responsibility to close the response body.
func (c *HTTPHeaderClient) Execute(execRequest execution.Request) (execution.Response, error) {
	return c.Client.Execute(execRequest)
}

type httpTransport struct {
	client  *http.Client
	headers http.Header
	url     *url.URL
}

func (t *httpTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.URL = t.url
	for key, values := range t.headers {
		for _, value := range values {
			r.Header.Add(key, value)
		}
	}
	return t.client.Do(r)
}
