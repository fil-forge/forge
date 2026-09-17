package redis

import (
	"github.com/fil-forge/indexing-service/pkg/types"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/fil-forge/ucantone/ucan/invocation"
	cid "github.com/ipfs/go-cid"
)

var _ types.ContentClaimsCache = (*ContentClaimsStore)(nil)

// ContentClaimsStore is a RedisStore for storing content claims that implements types.ContentClaimsStore
type ContentClaimsStore = Store[cid.Cid, ucan.Invocation]

// NewContentClaimsStore returns a new instance of a Content Claims Store using the given redis client
func NewContentClaimsStore(client Client, opts ...Option) *ContentClaimsStore {
	return NewStore(claimFromRedis, claimToRedis, cidKeyString, client, opts...)
}

func claimFromRedis(data string) (ucan.Invocation, error) {
	return invocation.Decode([]byte(data))
}

func claimToRedis(claim ucan.Invocation) (string, error) {
	data, err := invocation.Encode(claim)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func cidKeyString(c cid.Cid) string {
	return multihashKeyString(c.Hash())
}
