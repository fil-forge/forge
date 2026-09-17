package types

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/fil-forge/libforge/blobindex"
	contentcaps "github.com/fil-forge/libforge/commands/content"
	"github.com/fil-forge/ucantone/did"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/ipfs/go-cid"
	"github.com/ipni/go-libipni/find/model"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multicodec"
	mh "github.com/multiformats/go-multihash"
	cbg "github.com/whyrusleeping/cbor-gen"
)

// ContextID describes the data used to calculate a context id for IPNI
type ContextID struct {
	Space *did.DID
	Hash  mh.Multihash
}

// EncodedContextID is the encoded form of context id data that is actually stored
// in IPNI
type EncodedContextID []byte

// ToEncoded canonically encodes ContextID data
func (c ContextID) ToEncoded() (EncodedContextID, error) {
	if c.Space == nil {
		return EncodedContextID(c.Hash), nil
	}
	mh, err := mh.Sum(bytes.Join([][]byte{[]byte(c.Space.String()), c.Hash}, nil), mh.SHA2_256, -1)
	return EncodedContextID(mh), err
}

// ErrKeyNotFound means the key did not exist in the cache
var ErrKeyNotFound = errors.New("key not found")

// Store describes a generic storage interface
type Store[Key, Value any] interface {
	// Put adds (or replaces) an item in the store.
	Put(ctx context.Context, key Key, value Value) error
	// Get retrieves an existing item from the store. If the item does not exist,
	// it should return [ErrKeyNotFound].
	Get(ctx context.Context, key Key) (Value, error)
}

// ErrWrongRootCount indicates a car file with multiple roots being unable to interpret
// as a query result
var ErrWrongRootCount = errors.New("query result should have exactly one root")

// ErrNoRootBlock indicates a root that is specified but not found in a CAR file
var ErrNoRootBlock = errors.New("query root block not found in car")

// Cache describes a generic cache interface
type Cache[Key, Value any] interface {
	Set(ctx context.Context, key Key, value Value, expires bool) error
	SetExpirable(ctx context.Context, key Key, expires bool) error
	Get(ctx context.Context, key Key) (Value, error)
}

// ValueSetCache describes a cache interface whose values are sets
type ValueSetCache[Key, Value any] interface {
	Add(ctx context.Context, key Key, values ...Value) (uint64, error)
	SetExpirable(ctx context.Context, key Key, expires bool) error
	Members(ctx context.Context, key Key) ([]Value, error)
}

// BatchingValueSetCache is a value-set cache that can batch updates.
// Note: a batch is not a transaction.
type BatchingValueSetCache[Key, Value any] interface {
	ValueSetCache[Key, Value]
	Batch() ValueSetCacheBatcher[Key, Value]
}

type ValueSetCacheBatcher[Key, Value any] interface {
	Add(ctx context.Context, key Key, values ...Value) error
	SetExpirable(ctx context.Context, key Key, expires bool) error
	Commit(ctx context.Context) error
}

// ProviderStore caches queries to IPNI
type ProviderStore BatchingValueSetCache[mh.Multihash, model.ProviderResult]

// NoProviderStore caches which queries for providers returned no results
type NoProviderStore ValueSetCache[mh.Multihash, multicodec.Code]

// ContentClaimsStore stores published content claims
type ContentClaimsStore Store[cid.Cid, ucan.Invocation]

// ContentClaimsCache caches fetched content claims
type ContentClaimsCache Cache[cid.Cid, ucan.Invocation]

// ShardedDagIndexStore caches fetched sharded dag indexes
type ShardedDagIndexStore Cache[EncodedContextID, blobindex.ShardedDagIndex]

// Match narrows parameters for locating providers/claims for a set of multihashes
type Match struct {
	Subject []did.DID
}

// QueryType allows defining which claims a query is targeting. QueryTypeStandard targets all claims,
// i.e. Location, Index and Equals
type QueryType int

const (
	QueryTypeStandard QueryType = iota
	QueryTypeLocation
	QueryTypeIndexOrLocation
	QueryTypeStandardCompressed
)

func (qt QueryType) String() string {
	switch qt {
	case QueryTypeStandard:
		return "standard"
	case QueryTypeLocation:
		return "location"
	case QueryTypeIndexOrLocation:
		return "index_or_location"
	case QueryTypeStandardCompressed:
		return "standard_compressed"
	default:
		return "invalid"
	}
}

func ParseQueryType(queryTypeStr string) (QueryType, error) {
	switch queryTypeStr {
	case QueryTypeStandard.String():
		return QueryTypeStandard, nil
	case QueryTypeLocation.String():
		return QueryTypeLocation, nil
	case QueryTypeIndexOrLocation.String():
		return QueryTypeIndexOrLocation, nil
	case QueryTypeStandardCompressed.String():
		return QueryTypeStandardCompressed, nil
	default:
		return 0, fmt.Errorf("invalid query type: %s", queryTypeStr)
	}
}

// Query is a query for several multihashes
type Query struct {
	Type   QueryType
	Hashes []mh.Multihash
	Match  Match
	// Delegations allowing the indexer to retrieve bytes from the network. These
	// are typically `/content/retrieve` delegations for each subject (space)
	// in the [Match] parameter.
	//
	// Delegations are sent in the `X-UCAN-Container` HTTP header and MUST NOT
	// exceed 4kb in size.
	Delegations []ucan.Delegation
}

type Block struct {
	Link cid.Cid
	Data []byte
}

// QueryResult is an encodable result of a query
type QueryResult interface {
	Root() Block
	Blocks() []Block
	// Claims is a list of links to the root block of claims that can be found in this message
	Claims() []cid.Cid
	// Indexes is a list of links to the CID hash of archived sharded dag indexes that can be found in this
	// message
	Indexes() []cid.Cid
}

type Getter interface {
	// Get retrieves a claim that has been published or cached by the
	// indexing service. No external sources are consulted.
	Get(ctx context.Context, claim cid.Cid) (ucan.Invocation, error)
}

type Publisher interface {
	// Cache caches a claim with the service temporarily.
	Cache(ctx context.Context, provider peer.AddrInfo, claim ucan.Invocation, meta ucan.Container) error
	// Publish writes a claim to permanent storage, adds it to an IPNI
	// advertisement, annnounces it to IPNI nodes and caches it.
	Publish(ctx context.Context, claim ucan.Invocation, meta ucan.Container) error
}

type Querier interface {
	// Query allows claims to be queried by their subject (content CID). It
	// returns claims as well as any relevant indexes.
	Query(ctx context.Context, q Query) (QueryResult, error)
}

// Service is the core methods of the indexing service.
type Service interface {
	Getter
	Publisher
	Querier
}

// RetrievalRequest is all the details needed for retrieving data from the
// network. At minimum it contains the URL to retrieve a blob from.
//
// Legacy retrievals will not carry any retrieval authorization information i.e.
// the Auth field will be nil.
type RetrievalRequest struct {
	// URL where the blob may be requested from.
	URL url.URL
	// Optional byte range to request.
	Range *contentcaps.Range
	// UCAN authorization parameters.
	Auth RetrievalAuth
}

// RetrievalAuth are the details for a UCAN authorized content retrieval.
//
// The provided proofs are expected to contain, for example, a
// `/content/retrieve` delegated capability allowing content to be retrieved
// using UCAN authorization.
type RetrievalAuth struct {
	// The Indexing Service UCAN signing key.
	Issuer ucan.Issuer
	// Identity of the storage node to retrieve data from.
	Audience did.DID
	// Command for retrieval invocation.
	Command ucan.Command
	// Subject of the retrieval invocation.
	Subject did.DID
	// Arguments for the retrieval invocation.
	Arguments cbg.CBORMarshaler
	// Delegations from the client (e.g. `/content/retrieve`) or a storage
	// node (e.g. `/blob/retrieve`) to the indexing service authorizing retrieval.
	Proofs []ucan.Delegation
}
