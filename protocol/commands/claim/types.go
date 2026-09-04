package claim

import "github.com/ipfs/go-cid"

type CacheArguments struct {
	Claim    cid.Cid  `cborgen:"claim" dagjsongen:"claim"`
	Provider Provider `cborgen:"provider" dagjsongen:"provider"`
}

type Provider struct {
	// Addresses are byte encoded multiaddrs of the provider.
	Addresses [][]byte `cborgen:"addresses" dagjsongen:"addresses"`
}
