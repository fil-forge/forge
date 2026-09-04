package contentclaims

import (
	"context"
	"fmt"
	"net/url"

	"github.com/fil-forge/forge/indexing-service/pkg/types"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/ipfs/go-cid"
)

type ClaimService struct {
	store  types.ContentClaimsStore
	cache  types.ContentClaimsCache
	finder Finder
}

var _ Service = (*ClaimService)(nil)

func (cs *ClaimService) Cache(ctx context.Context, claim ucan.Invocation) error {
	return cs.cache.Set(ctx, claim.Link(), claim, true)
}

func (cs *ClaimService) Find(ctx context.Context, claim cid.Cid, url *url.URL) (ucan.Invocation, error) {
	return cs.finder.Find(ctx, claim, url)
}

func (cs *ClaimService) Get(ctx context.Context, claim cid.Cid) (ucan.Invocation, error) {
	c, err := cs.cache.Get(ctx, claim)
	if err == nil {
		return c, nil
	}
	if err != types.ErrKeyNotFound {
		return nil, err
	}
	c, err = cs.store.Get(ctx, claim)
	if err != nil {
		return nil, fmt.Errorf("getting claim from store: %w", err)
	}
	err = cs.Cache(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("caching claim: %w", err)
	}
	return c, nil
}

func (cs *ClaimService) Publish(ctx context.Context, claim ucan.Invocation) error {
	err := cs.store.Put(ctx, claim.Link(), claim)
	if err != nil {
		return fmt.Errorf("putting claim to store: %w", err)
	}
	return cs.Cache(ctx, claim)
}

func New(store types.ContentClaimsStore, cache types.ContentClaimsCache, finder Finder) *ClaimService {
	f := WithIdentityCids(WithCache(WithStore(finder, store), cache))
	return &ClaimService{store, cache, f}
}
