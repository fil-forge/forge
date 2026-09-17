package contentclaims

import (
	"context"
	"net/url"

	"github.com/fil-forge/ucantone/ucan"
	"github.com/fil-forge/ucantone/ucan/invocation"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
)

type identityCidFinder struct {
	finder Finder
}

var _ Finder = (*identityCidFinder)(nil)

// WithIdentityCids augments a ClaimFinder with claims retrieved automatically whenever an identity CID is used
func WithIdentityCids(finder Finder) Finder {
	return &identityCidFinder{finder}
}

// Find attempts to fetch a claim from either the permenant storage or via the provided URL
func (idf *identityCidFinder) Find(ctx context.Context, id cid.Cid, fetchURL *url.URL) (ucan.Invocation, error) {
	if id.Prefix().MhType == multihash.IDENTITY {
		dh, err := multihash.Decode(id.Hash())
		if err != nil {
			return nil, err
		}
		return invocation.Decode(dh.Digest)
	}

	// attempt to fetch the claim from the underlying claim finder
	return idf.finder.Find(ctx, id, fetchURL)
}
