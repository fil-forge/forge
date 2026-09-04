package contentclaims

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/fil-forge/forge/indexing-service/pkg/types"
	"github.com/fil-forge/ucantone/result"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/ipfs/go-cid"
)

type storeFinder struct {
	finder Finder
	store  types.ContentClaimsStore
}

var _ Finder = (*storeFinder)(nil)

// WithStore augments a ClaimFinder with claims retrieved from a claim store
func WithStore(finder Finder, store types.ContentClaimsStore) Finder {
	return &storeFinder{finder, store}
}

// Find attempts to fetch a claim from either the permenant storage or via the provided URL
func (sf *storeFinder) Find(ctx context.Context, id cid.Cid, fetchURL *url.URL) (ucan.Invocation, error) {

	// buffered channels so goroutines don't block.
	storeCh := make(chan result.Result[ucan.Invocation, error], 1)
	finderCh := make(chan result.Result[ucan.Invocation, error], 1)

	// Create a cancelable context for the store query.
	storeCtx, cancelStore := context.WithCancel(ctx)
	defer cancelStore()

	// Create a cancelable context for the finder query.
	finderCtx, cancelFinder := context.WithCancel(ctx)
	defer cancelFinder()

	// Start store query
	go func() {
		claim, err := sf.store.Get(storeCtx, id)
		if err != nil {
			storeCh <- result.Err[ucan.Invocation, error](err)
			return
		}
		storeCh <- result.OK[ucan.Invocation, error](claim)
	}()

	// Start finder query
	go func() {
		claim, err := sf.finder.Find(finderCtx, id, fetchURL)
		if err != nil {
			finderCh <- result.Err[ucan.Invocation, error](err)
			return
		}
		finderCh <- result.OK[ucan.Invocation, error](claim)
	}()

	var storeRes, finderRes result.Result[ucan.Invocation, error]

	// Wait for both responses.
	for range 2 {
		select {
		case storeRes = <-storeCh:
			if storeRes.IsOK() {
				cancelFinder()
			}
		case finderRes = <-finderCh:
			if finderRes.IsOK() {
				cancelStore()
			}
		}
	}

	if storeRes.IsOK() {
		return storeRes.Unpack()
	}
	if finderRes.IsOK() {
		return finderRes.Unpack()
	}
	_, storeErr := storeRes.Unpack()
	_, finderErr := finderRes.Unpack()
	// if an error occurred other than the claim not being in the store, return it
	if !errors.Is(storeErr, types.ErrKeyNotFound) {
		return nil, errors.Join(fmt.Errorf("reading from claim store: %w", storeErr), finderErr)
	}
	return nil, finderErr
}
