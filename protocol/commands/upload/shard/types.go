package shard

import "github.com/ipfs/go-cid"

type ListArguments struct {
	Root   cid.Cid `cborgen:"root" dagjsongen:"root"`
	Cursor *string `cborgen:"cursor,omitempty" dagjsongen:"cursor,omitempty"`
	Size   *uint64 `cborgen:"size,omitempty" dagjsongen:"size,omitempty"`
}

type ListOK struct {
	Cursor  *string   `cborgen:"cursor,omitempty" dagjsongen:"cursor,omitempty"`
	Size    uint64    `cborgen:"size" dagjsongen:"size"`
	Results []cid.Cid `cborgen:"results" dagjsongen:"results"`
}
