package contentclaims

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/fil-forge/ucantone/ucan"
	"github.com/fil-forge/ucantone/ucan/invocation"
	"github.com/ipfs/go-cid"
)

// simpleFinder is a read through cache for fetching content claims
type simpleFinder struct {
	httpClient *http.Client
}

var _ Finder = (*simpleFinder)(nil)

// NewSimpleFinder creates a new [Finder] with the provided HTTP client.
func NewSimpleFinder(httpClient *http.Client) Finder {
	return &simpleFinder{
		httpClient: httpClient,
	}
}

// Find attempts to fetch a claim from the provided URL.
func (sf *simpleFinder) Find(ctx context.Context, id cid.Cid, fetchURL *url.URL) (ucan.Invocation, error) {
	// attempt to fetch the claim from provided url
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fetchURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := sf.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch claim: %w", err)
	}
	defer resp.Body.Close()

	var claim invocation.Invocation
	if err := claim.UnmarshalCBOR(resp.Body); err != nil {
		return nil, fmt.Errorf("failed to unmarshal claim: %w", err)
	}
	if id != claim.Link() {
		return nil, fmt.Errorf("received claim: %s, does not match expected claim: %s", claim.Link(), id)
	}
	return &claim, nil
}
