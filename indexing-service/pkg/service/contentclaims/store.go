package contentclaims

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/fil-forge/go-ipni-tools/pkg/store"
	"github.com/fil-forge/indexing-service/pkg/types"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/fil-forge/ucantone/ucan/invocation"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
)

type bucketStore struct {
	bucket store.Store
}

func (bs *bucketStore) Get(ctx context.Context, key cid.Cid) (ucan.Invocation, error) {
	r, err := bs.bucket.Get(ctx, toKey(key))
	if err != nil {
		if store.IsNotFound(err) {
			return nil, types.ErrKeyNotFound
		}
		return nil, err
	}
	defer r.Close()

	var inv invocation.Invocation
	if err := inv.UnmarshalCBOR(r); err != nil {
		return nil, fmt.Errorf("failed to unmarshal invocation: %w", err)
	}
	return &inv, nil
}

func (bs *bucketStore) Put(ctx context.Context, key cid.Cid, value ucan.Invocation) error {
	b, err := invocation.Encode(value)
	if err != nil {
		return fmt.Errorf("failed to encode invocation: %w", err)
	}
	return bs.bucket.Put(ctx, toKey(key), uint64(len(b)), bytes.NewReader(b))
}

var _ types.ContentClaimsStore = (*bucketStore)(nil)

// NewStoreFromBucket creates a claims store from a bucket style interface.
func NewStoreFromBucket(bucket store.Store) types.ContentClaimsStore {
	return &bucketStore{bucket}
}

type dsStore struct {
	ds datastore.Datastore
}

func (d *dsStore) Get(ctx context.Context, key cid.Cid) (ucan.Invocation, error) {
	b, err := d.ds.Get(ctx, datastore.NewKey(toKey(key)))
	if err != nil {
		if errors.Is(err, datastore.ErrNotFound) {
			return nil, types.ErrKeyNotFound
		}
		return nil, err
	}
	return invocation.Decode(b)

}

func (d *dsStore) Put(ctx context.Context, key cid.Cid, value ucan.Invocation) error {
	b, err := invocation.Encode(value)
	if err != nil {
		return fmt.Errorf("failed to encode invocation: %w", err)
	}
	return d.ds.Put(ctx, datastore.NewKey(toKey(key)), b)
}

var _ types.ContentClaimsStore = (*dsStore)(nil)

func NewStoreFromDatastore(ds datastore.Datastore) types.ContentClaimsStore {
	return &dsStore{ds}
}

// toKey transforms the claim root CID into a string key.
func toKey(link cid.Cid) string {
	return fmt.Sprintf("%s/%s", link, link)
}
