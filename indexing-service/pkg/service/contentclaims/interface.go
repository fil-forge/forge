package contentclaims

import (
	"context"
	"net/url"

	"github.com/fil-forge/ucantone/ucan"
	"github.com/ipfs/go-cid"
)

type Finder interface {
	// Find and retrieve a claim via URL.
	Find(ctx context.Context, claim cid.Cid, fetchURL *url.URL) (ucan.Invocation, error)
}

type Service interface {
	// Get reads the claim from the cache, or from storage if not found.
	Get(ctx context.Context, claim cid.Cid) (ucan.Invocation, error)
	// Find attempts to read the claim from the cache, falling back to retrieving
	// it from storage and finally, if still not found, fetching it from the
	// provided URL. The result is stored in the cache.
	Find(ctx context.Context, claim cid.Cid, fetchURL *url.URL) (ucan.Invocation, error)
	// Cache writes the claim to the cache with default expiry.
	Cache(ctx context.Context, claim ucan.Invocation) error
	// Publish writes the claim to the cache, and adds it to storage.
	Publish(ctx context.Context, claim ucan.Invocation) error
}
