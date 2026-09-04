package index

import "github.com/ipfs/go-cid"

type AddArguments struct {
	// Index is a link to the Content Archive (CAR) containing the index.
	Index cid.Cid `cborgen:"index" dagjsongen:"index"`
}
