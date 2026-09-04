package upload

import "github.com/ipfs/go-cid"

type AddArguments struct {
	Root   cid.Cid   `cborgen:"root" dagjsongen:"root"`
	Shards []cid.Cid `cborgen:"shards" dagjsongen:"shards"`
	Index  *cid.Cid  `cborgen:"index,omitempty" dagjsongen:"index,omitempty"`
}

type RemoveArguments struct {
	Root cid.Cid `cborgen:"root" dagjsongen:"root"`
}

type ListArguments struct {
	Cursor *string `cborgen:"cursor,omitempty" dagjsongen:"cursor,omitempty"`
	Size   *uint64 `cborgen:"size,omitempty" dagjsongen:"size,omitempty"`
}

type ListOK struct {
	Cursor  *string          `cborgen:"cursor,omitempty" dagjsongen:"cursor,omitempty"`
	Size    uint64           `cborgen:"size" dagjsongen:"size"`
	Results []ListUploadItem `cborgen:"results" dagjsongen:"results"`
}

type ListUploadItem struct {
	Root  cid.Cid  `cborgen:"root" dagjsongen:"root"`
	Index *cid.Cid `cborgen:"index,omitempty" dagjsongen:"index,omitempty"`
}
