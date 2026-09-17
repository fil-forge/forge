package queryresult

import "github.com/ipfs/go-cid"

// QueryResult is the golang structure for encoding query results
type QueryResult struct {
	Result0_1 *QueryResult0_1 `cborgen:"index/query/result@0.1,omitempty" dagjsongen:"index/query/result@0.1,omitempty"`
}

// QueryResult0_1 describes the found claims and indexes for a given query
type QueryResult0_1 struct {
	Claims  []cid.Cid          `cborgen:"claims" dagjsongen:"claims"`
	Indexes map[string]cid.Cid `cborgen:"indexes" dagjsongen:"indexes"`
}
