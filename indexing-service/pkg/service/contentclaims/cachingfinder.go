package contentclaims

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/fil-forge/indexing-service/pkg/types"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/ipfs/go-cid"
)

type cachingFinder struct {
	finder Finder
	cache  types.ContentClaimsCache
}

var _ Finder = (*cachingFinder)(nil)

// WithCache augments a ClaimFinder with cached claims from a claim cache
func WithCache(finder Finder, cache types.ContentClaimsCache) Finder {
	return &cachingFinder{finder, cache}
}

// Find attempts to fetch a claim from either the local cache or via the provided URL (caching the result if its fetched)
func (cl *cachingFinder) Find(ctx context.Context, id cid.Cid, fetchURL *url.URL) (ucan.Invocation, error) {
	// attempt to read claim from cache and return it if succesful
	claim, err := cl.cache.Get(ctx, id)
	if err == nil {
		return claim, nil
	}

	// if an error occurred other than the claim not being in the cache, return it
	if !errors.Is(err, types.ErrKeyNotFound) {
		return nil, fmt.Errorf("reading from claim cache: %w", err)
	}

	// attempt to fetch the claim from the underlying claim finder
	claim, err = cl.finder.Find(ctx, id, fetchURL)
	if err != nil {
		return nil, err
	}

	// cache the claim for the future
	err = cl.cache.Set(ctx, id, claim, true)
	if err != nil {
		return nil, fmt.Errorf("caching claim: %w", err)
	}
	return claim, nil
}
