package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/fil-forge/forge/indexing-service/pkg/internal/jobwalker"
	"github.com/fil-forge/forge/indexing-service/pkg/internal/jobwalker/parallelwalk"
	"github.com/fil-forge/forge/indexing-service/pkg/internal/jobwalker/singlewalk"
	"github.com/fil-forge/forge/indexing-service/pkg/service/blobindexlookup"
	"github.com/fil-forge/forge/indexing-service/pkg/service/contentclaims"
	"github.com/fil-forge/forge/indexing-service/pkg/service/providerindex"
	"github.com/fil-forge/forge/indexing-service/pkg/service/queryresult"
	"github.com/fil-forge/forge/indexing-service/pkg/telemetry"
	"github.com/fil-forge/forge/indexing-service/pkg/types"
	"github.com/fil-forge/forge/internal/ipni/advertisement"
	"github.com/fil-forge/forge/protocol/blobindex"
	"github.com/fil-forge/forge/protocol/bytemap"
	"github.com/fil-forge/forge/protocol/commands/assert"
	"github.com/fil-forge/forge/protocol/commands/content"
	"github.com/fil-forge/forge/protocol/digestutil"
	"github.com/fil-forge/go-ipni-tools/pkg/metadata"
	"github.com/fil-forge/ucantone/did"
	ed25519_verifier "github.com/fil-forge/ucantone/multikey/ed25519/verifier"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/fil-forge/ucantone/ucan/container"
	ucanlib "github.com/fil-forge/ucantone/ucanlib"
	"github.com/fil-forge/ucantone/validator"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipni/go-libipni/find/model"
	"github.com/ipni/go-libipni/maurl"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
	"go.opentelemetry.io/otel/attribute"
)

const (
	ClaimUrlPlaceholder   = "{claim}"
	blobUrlPlaceholder    = "{blob}"
	blobCIDUrlPlaceholder = "{blobCID}"
)

var log = logging.Logger("service")

var ErrUnrecognizedClaim = errors.New("unrecognized claim type")

// IndexingService implements read/write logic for indexing data with IPNI, content claims, sharded dag indexes, and a cache layer
type IndexingService struct {
	id              ucan.Issuer
	blobIndexLookup blobindexlookup.BlobIndexLookup
	claims          contentclaims.Service
	providerIndex   providerindex.ProviderIndex
	// provider is the peer info for this service, used when publishing claims.
	provider  peer.AddrInfo
	jobWalker jobwalker.JobWalker[job, queryState]
}

var _ types.Service = (*IndexingService)(nil)

type job struct {
	mh                  multihash.Multihash
	indexForMh          *multihash.Multihash
	indexProviderRecord *model.ProviderResult
	queryType           types.QueryType
}

type jobKey string

func (j job) key() jobKey {
	k := jobKey(j.mh) + jobKey(j.queryType.String())
	if j.indexForMh != nil {
		k += jobKey(*j.indexForMh)
	}
	return k
}

var targetClaims = map[types.QueryType][]multicodec.Code{
	types.QueryTypeStandard:           {metadata.EqualsClaimID, metadata.IndexClaimID, metadata.LocationCommitmentID},
	types.QueryTypeStandardCompressed: {metadata.EqualsClaimID, metadata.IndexClaimID, metadata.LocationCommitmentID},
	types.QueryTypeLocation:           {metadata.LocationCommitmentID},
	types.QueryTypeIndexOrLocation:    {metadata.IndexClaimID, metadata.LocationCommitmentID},
}

type queryResult struct {
	Claims  map[cid.Cid]ucan.Invocation
	Indexes bytemap.ByteMap[types.EncodedContextID, blobindex.ShardedDagIndex]
}

type queryState struct {
	q      *types.Query
	qr     *queryResult
	visits map[jobKey]struct{}
}

func (is *IndexingService) jobHandler(mhCtx context.Context, j job, spawn func(job) error, state jobwalker.WrappedState[queryState]) error {
	mhCtx, s := telemetry.StartSpan(mhCtx, "IndexingService.jobHandler")
	defer s.End()
	s.SetAttributes(attribute.String("multihash", digestutil.Format(j.mh)))

	// check if node has already been visited and ignore if that is the case
	if !state.CmpSwap(func(qs queryState) bool {
		_, ok := qs.visits[j.key()]
		return !ok
	}, func(qs queryState) queryState {
		qs.visits[j.key()] = struct{}{}
		return qs
	}) {
		return nil
	}

	// find provider records related to this multihash
	s.AddEvent("finding relevant ProviderResults")
	results, err := is.providerIndex.Find(mhCtx, providerindex.QueryKey{
		Hash:         j.mh,
		Spaces:       state.Access().q.Match.Subject,
		TargetClaims: targetClaims[j.queryType],
	})
	if err != nil {
		telemetry.Error(s, err, "finding ProviderResults")
		return fmt.Errorf("finding provider results: %w", err)
	}

	s.AddEvent(fmt.Sprintf("processing %d results", len(results)))

	var indexFetchSucceeded bool
	var lastIndexFetchErr error

	for _, result := range results {
		// unmarshall metadata for this provider
		md := metadata.MetadataContext.New()
		err = md.UnmarshalBinary(result.Metadata)
		if err != nil {
			telemetry.Error(s, err, "unmarshaling metadata")
			return fmt.Errorf("unmarshaling metadata: %w", err)
		}

		// the provider may list one or more protocols for this CID
		// in our case, the protocols are just differnt types of content claims
		for _, code := range md.Protocols() {
			protocol := md.Get(code)
			// make sure this is some kind of claim protocol, ignore if not
			hasClaimCid, ok := protocol.(metadata.HasClaim)
			if !ok {
				continue
			}

			// fetch (from cache or url) the actual content claim
			claimCid := hasClaimCid.GetClaim()
			log.Infow("query: fetching claim", "claimCid", claimCid, "providerId", result.Provider.ID, "numAddrs", len(result.Provider.Addrs))
			for i, addr := range result.Provider.Addrs {
				log.Infow("query: provider address", "index", i, "addr", addr.String())
			}
			url, err := fetchClaimURL(*result.Provider, claimCid)
			if err != nil {
				log.Errorw("query: failed to build claim URL", "error", err)
				telemetry.Error(s, err, "building claim URL")
				return fmt.Errorf("fetching claim URL: %w", err)
			}

			s.AddEvent("fetching claims")
			claim, err := is.claims.Find(mhCtx, claimCid, url)
			if err != nil {
				telemetry.Error(s, err, "fetching claims")
				return fmt.Errorf("fetching claims: %w", err)
			}
			// add the fetched claim to the results, if we don't already have it
			state.CmpSwap(
				func(qs queryState) bool {
					_, ok := qs.qr.Claims[claimCid]
					return !ok
				},
				func(qs queryState) queryState {
					qs.qr.Claims[claimCid] = claim
					return qs
				})

			// handle each type of protocol
			switch typedProtocol := protocol.(type) {
			case *metadata.EqualsClaimMetadata:
				s.AddEvent("processing equals claim")

				// for an equals claim, it's published on both the content and equals multihashes
				// we follow with a query for location claim on the OTHER side of the multihash
				if string(typedProtocol.Equals.Hash()) != string(j.mh) {
					// lookup was the content hash, queue the equals hash
					if err := spawn(job{typedProtocol.Equals.Hash(), nil, nil, types.QueryTypeLocation}); err != nil {
						telemetry.Error(s, err, "queing job for equals hash")
						return fmt.Errorf("queuing job for equals hash: %w", err)
					}
				} else {
					// lookup was the equals hash, queue the content hash
					if err := spawn(job{multihash.Multihash(result.ContextID), nil, nil, types.QueryTypeLocation}); err != nil {
						telemetry.Error(s, err, "queuing job for content hash")
						return fmt.Errorf("queuing job for content hash: %w", err)
					}
				}
			case *metadata.IndexClaimMetadata:
				s.AddEvent("processing index claim")

				// for an index claim, we follow by looking for a location claim for the index, and fetching the index
				mh := j.mh
				if err := spawn(job{typedProtocol.Index.Hash(), &mh, &result, types.QueryTypeLocation}); err != nil {
					telemetry.Error(s, err, "queuing job for the index's location claim")
					return fmt.Errorf("queuing job for index location claim: %w", err)
				}
			case *metadata.LocationCommitmentMetadata:
				s.AddEvent("processing location claim")

				// for a location claim, we just store it, unless its for an index CID, in which case get the full index
				if j.indexForMh != nil {
					// Try to fetch the index from this provider result
					// If it fails, we'll continue to the next result instead of failing the entire query
					shard := typedProtocol.Shard
					if shard == nil {
						c := cid.NewCidV1(cid.Raw, j.mh)
						shard = &c
					}
					url, err := fetchRetrievalURL(*result.Provider, *shard)
					if err != nil {
						telemetry.Error(s, err, "fetching index retrieval URL")
						log.Warnw("failed to fetch retrieval URL, will try next provider result if available", "shard", shard, "provider", result.Provider.ID, "err", err)
						lastIndexFetchErr = fmt.Errorf("fetching retrieval URL for index %q from provider %s: %w", shard, result.Provider.ID, err)
						continue // Try next provider result
					}

					s.AddEvent("fetching index")
					var lcArgs assert.LocationArguments
					if err := lcArgs.UnmarshalCBOR(bytes.NewReader(claim.ArgumentsBytes())); err != nil {
						log.Warnw("failed to match claim to location commitment, will try next provider result if available", "err", err)
						lastIndexFetchErr = fmt.Errorf("failed to match claim to location commitment: %w", err)
						continue
					}

					space := lcArgs.Space
					dlgs := state.Access().q.Delegations
					// Authorized retrieval requires a space in the location claim, a
					// delegation for the retrieval, and an absolute byte range to extract.
					if lcArgs.Range == nil || lcArgs.Range.End == nil {
						log.Warnw("missing range in location commitment, will try next provider result if available", "err", err)
						lastIndexFetchErr = fmt.Errorf("missing range in location commitment for blob: %s", shard)
						continue
					}

					proofStore := ucanlib.NewContainerProofStore(container.New(container.WithDelegations(dlgs...)))
					proofs, _, err := proofStore.ProofChain(mhCtx, is.id.DID(), content.Retrieve.Command, space)
					if err != nil {
						log.Warnw("failed to build proof chain, will try next provider result if available", "err", err)
						lastIndexFetchErr = fmt.Errorf("failed to build proof chain: %w", err)
						continue
					}
					// ProofChain returns (nil, nil) when no chain matches; the
					// proofs[0].Subject() access below would otherwise panic.
					// Skip this provider — a later result might supply a usable
					// chain, or the loop will fall through with a useful error.
					if len(proofs) == 0 {
						log.Warnw("no /content/retrieve proof chain in query delegations, will try next provider result if available",
							"audience", is.id.DID().String(),
							"subject", space.String(),
						)
						lastIndexFetchErr = fmt.Errorf("no /content/retrieve proof chain for audience %s subject %s", is.id.DID(), space)
						continue
					}

					contentRange := content.Range{
						Start: lcArgs.Range.Start,
						End:   *lcArgs.Range.End,
					}
					req := types.RetrievalRequest{
						URL:   *url,
						Range: &contentRange,
						Auth: types.RetrievalAuth{
							Issuer:   is.id,
							Audience: claim.Issuer(),
							Command:  content.Retrieve.Command,
							Subject:  proofs[0].Subject(),
							Arguments: &content.RetrieveArguments{
								Blob:  content.Blob{Digest: lcArgs.Content},
								Range: contentRange,
							},
							Proofs: proofs,
						},
					}

					index, err := is.blobIndexLookup.Find(mhCtx, result.ContextID, *j.indexProviderRecord, req)
					if err != nil {
						telemetry.Error(s, err, "fetching index blob")
						log.Warnw("failed to fetch index blob, will try next provider result if available", "provider", result.Provider.ID, "err", err)
						lastIndexFetchErr = fmt.Errorf("fetching index blob from provider %s: %w", result.Provider.ID, err)
						continue // Try next provider result
					}

					// Success! Add the index to the query results, if we don't already have it
					indexFetchSucceeded = true
					state.CmpSwap(
						func(qs queryState) bool {
							return !qs.qr.Indexes.Has(result.ContextID)
						},
						func(qs queryState) queryState {
							qs.qr.Indexes.Set(result.ContextID, index)
							return qs
						})

					// add location queries for all shards containing the original CID we're seeing an index for
					s.AddEvent("adding location queries for indexed shards")
					shards := index.Shards().Iterator()
					for shard, index := range shards {
						if index.Has(*j.indexForMh) {
							if err := spawn(job{shard, nil, nil, types.QueryTypeLocation}); err != nil {
								telemetry.Error(s, err, "queuing location job for shard")
								return fmt.Errorf("queuing location job for shard: %w", err)
							}
						}
					}
				}
			}
		}
	}

	// If we attempted to fetch an index but all attempts failed, return the last error
	if lastIndexFetchErr != nil && !indexFetchSucceeded {
		return fmt.Errorf("failed to fetch index from all provider results: %w", lastIndexFetchErr)
	}
	return nil
}

// Query returns back relevant content claims for the given query using the following steps
// 1. Query the ProviderIndex for all matching records
// 2. For any index claims, query the ProviderIndex for location claims for the index cid
// 3. Query the BlobIndexLookup to get the full ShardedDagIndex for any index claims
// 4. Query ProviderIndex for any location claims for any shards that contain the multihash based on the ShardedDagIndex
// 5. Read the requisite claims from the ClaimLookup
// 6. Return all discovered claims and sharded dag indexes
func (is *IndexingService) Query(ctx context.Context, q types.Query) (types.QueryResult, error) {
	ctx, s := telemetry.StartSpan(ctx, "IndexingService.Query")
	defer s.End()

	if q.Type == types.QueryTypeStandardCompressed && len(q.Hashes) != 1 {
		return nil, fmt.Errorf("invalid query: expected 1 hash for compressed query, got %d", len(q.Hashes))
	}

	initialJobs := make([]job, 0, len(q.Hashes))
	for _, mh := range q.Hashes {
		initialJobs = append(initialJobs, job{mh, nil, nil, q.Type})
	}
	qs, err := is.jobWalker(ctx, initialJobs, queryState{
		q: &q,
		qr: &queryResult{
			Claims:  make(map[cid.Cid]ucan.Invocation),
			Indexes: bytemap.NewByteMap[types.EncodedContextID, blobindex.ShardedDagIndex](-1),
		},
		visits: map[jobKey]struct{}{},
	}, is.jobHandler)
	if err != nil {
		return nil, err
	}
	if q.Type == types.QueryTypeStandardCompressed {
		return queryresult.BuildCompressed(q.Hashes[0], is.id, qs.qr.Claims, qs.qr.Indexes)
	}
	return queryresult.Build(qs.qr.Claims, qs.qr.Indexes)
}

type replacement struct {
	resourcePlaceholder string
	resourceID          string
}

func urlForResource(provider peer.AddrInfo, replacements []replacement) (*url.URL, error) {
	for _, addr := range provider.Addrs {
		// first, attempt to convert the addr to a url scheme
		url, err := maurl.ToURL(addr)
		// if it can't be converted, skip
		if err != nil {
			continue
		}
		// must be an http url
		if !(url.Scheme == "http" || url.Scheme == "https") {
			continue
		}
		replacedAny := false
		for _, replacement := range replacements {
			resourcePlaceholder, resourceID := replacement.resourcePlaceholder, replacement.resourceID
			// we must have a place to place the resourceId in the path
			if !strings.Contains(url.Path, resourcePlaceholder) {
				continue
			}
			replacedAny = true
			// ok we have a matching URL, return with all resource placeholders replaced with the id
			url.Path = strings.ReplaceAll(url.Path, resourcePlaceholder, resourceID)
		}
		if replacedAny {
			// maurl.ToURL produces a path without a leading slash because
			// multiaddrs are path-segment based. URL.String() papers this
			// over when rendering, but URL.RequestURI() (which
			// http.Request.Write uses on the wire) reproduces the path
			// verbatim — and an HTTP/1.1 request-target without a leading
			// slash is malformed, causing Go's net/http server to reject
			// it with a bare "400 Bad Request" before any handler runs.
			// Normalize here.
			if !strings.HasPrefix(url.Path, "/") {
				url.Path = "/" + url.Path
			}
			return url, nil
		}
	}

	placeholders := make([]string, 0, len(replacements))
	for _, r := range replacements {
		placeholders = append(placeholders, r.resourcePlaceholder)
	}

	addrs := make([]string, 0, len(provider.Addrs))
	for _, addr := range provider.Addrs {
		addrs = append(addrs, addr.String())
	}

	return nil, fmt.Errorf("no %s endpoint found in %d addresses: %s", strings.Join(placeholders, " or "), len(addrs), strings.Join(addrs, ", "))
}

func fetchClaimURL(provider peer.AddrInfo, claimCid cid.Cid) (*url.URL, error) {
	return urlForResource(provider, []replacement{{ClaimUrlPlaceholder, claimCid.String()}})
}

func fetchRetrievalURL(provider peer.AddrInfo, shard cid.Cid) (*url.URL, error) {
	return urlForResource(provider, []replacement{{blobUrlPlaceholder, digestutil.Format(shard.Hash())}, {blobCIDUrlPlaceholder, shard.String()}})
}

func (is *IndexingService) Get(ctx context.Context, claim cid.Cid) (ucan.Invocation, error) {
	return is.claims.Get(ctx, claim)
}

// Cache is used to cache a claim without publishing it to IPNI
// this is used cache a location commitment that come from a storage provider on blob/accept, without publishing, since the SP will publish themselves
// (a delegation for a location commitment is already generated on blob/accept)
// ideally however, IPNI would enable UCAN chains for publishing so that we could publish it directly from the storage service
// it doesn't for now, so we let SPs publish themselves them direct cache with us
func (is *IndexingService) Cache(ctx context.Context, provider peer.AddrInfo, claim ucan.Invocation, meta ucan.Container) error {
	return Cache(ctx, is.blobIndexLookup, is.claims, is.providerIndex, provider, claim, meta)
}

// Publish caches and publishes a content claim
// I imagine publish claim to work as follows
// For all claims except index, just use the publish API on ProviderIndex
// For index claims, let's assume they fail if a location claim for the index car cid is not already published
// The service should lookup the index cid location claim, and fetch the ShardedDagIndexView, then use the hashes inside
// to assemble all the multihashes in the index advertisement
func (is *IndexingService) Publish(ctx context.Context, claim ucan.Invocation, meta ucan.Container) error {
	return Publish(ctx, is.id, is.blobIndexLookup, is.claims, is.providerIndex, is.provider, claim, meta)
}

// Option configures an IndexingService
type Option func(is *IndexingService)

// WithConcurrency causes the indexing service to process find queries parallel, with the given concurrency
func WithConcurrency(concurrency int) Option {
	return func(is *IndexingService) {
		is.jobWalker = parallelwalk.NewParallelWalk[job, queryState](concurrency)
	}
}

// NewIndexingService returns a new indexing service
func NewIndexingService(id ucan.Issuer, blobIndexLookup blobindexlookup.BlobIndexLookup, claims contentclaims.Service, publicAddrInfo peer.AddrInfo, providerIndex providerindex.ProviderIndex, options ...Option) *IndexingService {
	provider := peer.AddrInfo{ID: publicAddrInfo.ID}
	for _, addr := range publicAddrInfo.Addrs {
		claimSuffix, _ := multiaddr.NewMultiaddr("/http-path/" + url.PathEscape("claim/"+ClaimUrlPlaceholder))
		provider.Addrs = append(provider.Addrs, multiaddr.Join(addr, claimSuffix))
	}
	is := &IndexingService{
		id:              id,
		blobIndexLookup: blobIndexLookup,
		claims:          claims,
		provider:        provider,
		providerIndex:   providerIndex,
		jobWalker:       singlewalk.SingleWalker[job, queryState],
	}
	for _, option := range options {
		option(is)
	}
	return is
}

func Cache(ctx context.Context, blobIndex blobindexlookup.BlobIndexLookup, claims contentclaims.Service, provIndex providerindex.ProviderIndex, provider peer.AddrInfo, claim ucan.Invocation, meta ucan.Container) error {
	ctx, s := telemetry.StartSpan(ctx, "IndexingService.Cache")
	defer s.End()

	switch claim.Command() {
	case assert.Location.Command:
		s.SetAttributes(attribute.KeyValue{Key: "claim", Value: attribute.StringValue("assert/location")})
		return cacheLocationCommitment(ctx, claims, provIndex, provider, claim, meta)
	default:
		return ErrUnrecognizedClaim
	}
}

func cacheLocationCommitment(ctx context.Context, claims contentclaims.Service, provIndex providerindex.ProviderIndex, provider peer.AddrInfo, claim ucan.Invocation, _ ucan.Container) error {
	if claim.Command() != assert.Location.Command {
		return fmt.Errorf("invalid claim type: expected assert/location, got %s", claim.Command().String())
	}

	var args assert.LocationArguments
	if err := args.UnmarshalCBOR(bytes.NewReader(claim.ArgumentsBytes())); err != nil {
		return fmt.Errorf("unmarshaling location claim arguments: %w", err)
	}

	err := claims.Cache(ctx, claim)
	if err != nil {
		return fmt.Errorf("caching claim with claim service: %w", err)
	}

	digests := []multihash.Multihash{args.Content}
	contextID, err := advertisement.EncodeContextID(args.Space, args.Content)
	if err != nil {
		return fmt.Errorf("encoding advertisement context ID: %w", err)
	}

	var exp ucan.UnixTimestamp
	if claim.Expiration() != nil {
		exp = *claim.Expiration()
	}

	var rng *metadata.Range
	if args.Range != nil {
		rng = &metadata.Range{Offset: args.Range.Start}
		if args.Range.End != nil {
			length := *args.Range.End - args.Range.Start + 1
			rng.Length = &length
		}
	}

	shardCid, err := advertisement.ShardCID(provider, args)
	if err != nil {
		return fmt.Errorf("failed to extract shard CID for provider: %s locationCommitment %v: %w", provider, args, err)
	}

	meta := metadata.MetadataContext.New(
		&metadata.LocationCommitmentMetadata{
			Shard:      shardCid,
			Expiration: int64(exp),
			Claim:      claim.Link(),
			Range:      rng,
		},
	)

	err = provIndex.Cache(ctx, provider, string(contextID), slices.Values(digests), meta)
	if err != nil {
		return fmt.Errorf("caching claim with provider index: %w", err)
	}

	return nil
}

func Publish(ctx context.Context, id ucan.Issuer, blobIndex blobindexlookup.BlobIndexLookup, claims contentclaims.Service, provIndex providerindex.ProviderIndex, provider peer.AddrInfo, claim ucan.Invocation, meta ucan.Container) error {
	ctx, s := telemetry.StartSpan(ctx, "IndexingService.Publish")
	defer s.End()

	switch claim.Command() {
	case assert.Equals.Command:
		s.SetAttributes(attribute.KeyValue{Key: "claim", Value: attribute.StringValue(assert.Equals.String())})
		return publishEqualsClaim(ctx, claims, provIndex, provider, claim, meta)
	case assert.Index.Command:
		s.SetAttributes(attribute.KeyValue{Key: "claim", Value: attribute.StringValue(assert.Index.String())})
		return publishIndexClaim(ctx, id, blobIndex, claims, provIndex, provider, claim, meta)
	default:
		return ErrUnrecognizedClaim
	}
}

func publishEqualsClaim(ctx context.Context, claims contentclaims.Service, provIndex providerindex.ProviderIndex, provider peer.AddrInfo, claim ucan.Invocation, _ ucan.Container) error {
	var args assert.EqualsArguments
	if err := args.UnmarshalCBOR(bytes.NewReader(claim.ArgumentsBytes())); err != nil {
		return fmt.Errorf("unmarshaling equals claim arguments: %w", err)
	}

	err := claims.Publish(ctx, claim)
	if err != nil {
		return fmt.Errorf("caching equals claim with claim service: %w", err)
	}

	var exp ucan.UnixTimestamp
	if claim.Expiration() != nil {
		exp = *claim.Expiration()
	}

	meta := metadata.MetadataContext.New(
		&metadata.EqualsClaimMetadata{
			Equals:     args.Equals,
			Expiration: int64(exp),
			Claim:      claim.Link(),
		},
	)

	var digests []multihash.Multihash
	digests = append(digests, args.Content)
	digests = append(digests, args.Equals.Hash())
	contextID := string(args.Content)
	err = provIndex.Publish(ctx, provider, contextID, slices.Values(digests), meta)
	if err != nil {
		return fmt.Errorf("publishing equals claim: %w", err)
	}

	return nil
}

func publishIndexClaim(ctx context.Context, id ucan.Issuer, blobIndex blobindexlookup.BlobIndexLookup, claims contentclaims.Service, provIndex providerindex.ProviderIndex, provider peer.AddrInfo, claim ucan.Invocation, claimMeta ucan.Container) error {
	var args assert.IndexArguments
	if err := args.UnmarshalCBOR(bytes.NewReader(claim.ArgumentsBytes())); err != nil {
		return fmt.Errorf("unmarshaling index claim arguments: %w", err)
	}

	err := claims.Publish(ctx, claim)
	if err != nil {
		return fmt.Errorf("caching index claim with claim lookup: %w", err)
	}

	results, err := provIndex.Find(ctx, providerindex.QueryKey{
		Hash:         args.Index.Hash(),
		TargetClaims: []multicodec.Code{metadata.LocationCommitmentID},
	})
	if err != nil {
		return fmt.Errorf("finding location commitment: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("no location commitments found for index: %s", args.Index)
	}

	var idx blobindex.ShardedDagIndex
	var ferr error
	for _, r := range results {
		idx, ferr = fetchBlobIndex(ctx, id, blobIndex, claims, args.Index, r, claim, claimMeta)
		if ferr != nil {
			continue
		}
		break
	}
	if ferr != nil {
		return fmt.Errorf("fetching blob index: %w", ferr)
	}

	var exp ucan.UnixTimestamp
	if claim.Expiration() != nil {
		exp = *claim.Expiration()
	}

	meta := metadata.MetadataContext.New(
		&metadata.IndexClaimMetadata{
			Index:      args.Index,
			Expiration: int64(exp),
			Claim:      claim.Link(),
		},
	)

	digests := bytemap.NewByteMap[multihash.Multihash, struct{}](-1)
	for _, slices := range idx.Shards().Iterator() {
		for d := range slices.Iterator() {
			digests.Set(d, struct{}{})
		}
	}

	contextID := args.Index.Bytes()
	err = provIndex.Publish(ctx, provider, string(contextID), digests.Keys(), meta)
	if err != nil {
		return fmt.Errorf("publishing index claim: %w", err)
	}

	return nil
}

func fetchBlobIndex(
	ctx context.Context,
	id ucan.Issuer,
	blobIndex blobindexlookup.BlobIndexLookup,
	claims contentclaims.Service,
	blobLink cid.Cid,
	result model.ProviderResult,
	cause ucan.Invocation, // supporting context (typically `/assert/index`)
	causeMeta ucan.Container,
) (blobindex.ShardedDagIndex, error) {
	meta := metadata.MetadataContext.New()
	err := meta.UnmarshalBinary(result.Metadata)
	if err != nil {
		return nil, fmt.Errorf("decoding location commitment metadata: %w", err)
	}

	protocol := meta.Get(metadata.LocationCommitmentID)
	lcmeta, ok := protocol.(*metadata.LocationCommitmentMetadata)
	if !ok {
		return nil, errors.New("metadata is not expected type")
	}

	if lcmeta.Shard != nil {
		blobLink = *lcmeta.Shard
	}

	blobURL, err := fetchRetrievalURL(*result.Provider, blobLink)
	if err != nil {
		return nil, fmt.Errorf("building retrieval URL: %w", err)
	}

	aud, err := peerToPrincipal(result.Provider.ID)
	if err != nil {
		return nil, fmt.Errorf("converting provider peer ID to UCAN principal: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	var validateErr error
	go func() {
		defer wg.Done()
		claimURL, err := fetchClaimURL(*result.Provider, lcmeta.Claim)
		if err != nil {
			validateErr = fmt.Errorf("building claim URL: %w", err)
			return
		}

		claim, err := claims.Find(ctx, lcmeta.Claim, claimURL)
		if err != nil {
			validateErr = err
			return
		}

		err = validateLocationCommitment(ctx, claim)
		if err != nil {
			validateErr = err
			return
		}
	}()

	// Try to extract a /content/retrieve delegation from the /assert/index
	// invocation.
	proofs, err := extractContentRetrieveDelegation(ctx, id.DID(), cause, causeMeta)
	if err != nil {
		return nil, err
	}

	byteRange := lcmeta.Range
	if byteRange == nil || byteRange.Length == nil {
		return nil, fmt.Errorf("missing byte range in location commitment metadata for index: %s", blobLink)
	}
	contentRange := content.Range{
		Start: byteRange.Offset,
		End:   byteRange.Offset + *byteRange.Length - 1,
	}
	req := types.RetrievalRequest{
		URL:   *blobURL,
		Range: &contentRange,
		Auth: types.RetrievalAuth{
			Issuer:   id,
			Audience: aud,
			Command:  content.Retrieve.Command,
			Subject:  proofs[0].Subject(),
			Arguments: &content.RetrieveArguments{
				Blob:  content.Blob{Digest: blobLink.Hash()},
				Range: contentRange,
			},
			Proofs: proofs,
		},
	}
	// Note: the ContextID here is of a location commitment provider
	idx, err := blobIndex.Find(ctx, result.ContextID, result, req)
	if err != nil {
		return nil, fmt.Errorf("fetching index: %w", err)
	}

	wg.Wait()
	if validateErr != nil {
		return nil, fmt.Errorf("verifying claim: %w", validateErr)
	}

	return idx, nil
}

// validateLocationCommitment ensures that the claim is a valid UCAN (signed,
// not expired etc.) and is a location commitment.
func validateLocationCommitment(ctx context.Context, claim ucan.Invocation) error {
	if claim.Command() != assert.Location.Command {
		return fmt.Errorf("unexpected command: %s", claim.Command())
	}
	return validator.ValidateInvocation(ctx, claim)
}

// peerToPrincipal converts a peer ID into a `did:key:`. Currently
// supports only ed25519 keys.
func peerToPrincipal(peer peer.ID) (did.DID, error) {
	pk, err := peer.ExtractPublicKey()
	if err != nil {
		return did.Undef, fmt.Errorf("extracting public key from peer ID: %w", err)
	}
	pubBytes, err := pk.Raw()
	if err != nil {
		return did.Undef, fmt.Errorf("extracting raw bytes of public key: %w", err)
	}
	v, err := ed25519_verifier.FromRaw(pubBytes)
	if err != nil {
		return did.Undef, fmt.Errorf("decoding raw ed25519 public key: %w", err)
	}
	return v.KeyDID(), nil
}

// extractContentRetrieveDelegation extracts a `/content/retrieve`
// delegation attached to the passed invocation (typically an `/assert/index`).
// The delegations are expected to be linked from metadata by a "retrievalAuth"
// key.
func extractContentRetrieveDelegation(ctx context.Context, audience did.DID, assertion ucan.Invocation, assertionMeta ucan.Container) ([]ucan.Delegation, error) {
	var meta assert.IndexMetadata
	if err := meta.UnmarshalCBOR(bytes.NewReader(assertion.MetadataBytes())); err != nil {
		return nil, fmt.Errorf("unmarshaling assertion metadata: %w", err)
	}

	if len(meta.RetrievalAuth) == 0 {
		return nil, fmt.Errorf("missing content retrieval authorization")
	}

	dlgs := map[cid.Cid]ucan.Delegation{}
	for _, d := range assertionMeta.Delegations() {
		dlgs[d.Link()] = d
	}

	rootDelegation, ok := dlgs[meta.RetrievalAuth[0]]
	if !ok {
		return nil, fmt.Errorf("root retrieval auth delegation not found in index assertion metadata delegations: %s", meta.RetrievalAuth[0])
	}

	proofStore := ucanlib.NewContainerProofStore(assertionMeta)
	proofs, _, err := proofStore.ProofChain(ctx, audience, content.Retrieve.Command, rootDelegation.Subject())
	if err != nil {
		return nil, fmt.Errorf("building proof chain: %w", err)
	}

	// ProofChain yields zero delegations (with no error) when no chain
	// matches; downstream callers index proofs[0] unconditionally, so guard
	// here to return an error instead of letting them panic.
	if len(proofs) == 0 {
		return nil, fmt.Errorf("no /content/retrieve proof chain found for audience %s subject %s (root retrieval auth: %s)", audience, rootDelegation.Subject(), meta.RetrievalAuth[0])
	}

	return proofs, nil
}
