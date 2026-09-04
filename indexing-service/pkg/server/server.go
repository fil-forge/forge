package server

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/fil-forge/automobile"
	"github.com/fil-forge/forge/indexing-service/pkg/build"
	"github.com/fil-forge/forge/indexing-service/pkg/service/contentclaims"
	"github.com/fil-forge/forge/indexing-service/pkg/telemetry"
	"github.com/fil-forge/forge/indexing-service/pkg/types"
	"github.com/fil-forge/forge/internal/identity"
	assertcaps "github.com/fil-forge/forge/protocol/commands/assert"
	"github.com/fil-forge/forge/protocol/retrieval"
	"github.com/fil-forge/ucantone/did"
	"github.com/fil-forge/ucantone/multikey"
	"github.com/fil-forge/ucantone/multikey/ed25519"
	"github.com/fil-forge/ucantone/server"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/fil-forge/ucantone/ucan/container"
	"github.com/fil-forge/ucantone/ucan/invocation"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipni/go-libipni/find/model"
	"github.com/ipni/go-libipni/metadata"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multibase"
	"github.com/multiformats/go-multihash"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var log = logging.Logger("server")

type ipniConfig struct {
	provider peer.AddrInfo
	metadata []byte
}

type config struct {
	id                   identity.Identity
	contentClaimsOptions []server.HTTPOption
	enableTelemetry      bool
	ipniConfig           *ipniConfig
}

type Option func(*config) error

// WithIdentity specifies the server DID.
func WithIdentity(id identity.Identity) Option {
	return func(c *config) error {
		c.id = id
		return nil
	}
}

func WithContentClaimsOptions(options ...server.HTTPOption) Option {
	return func(c *config) error {
		c.contentClaimsOptions = options
		return nil
	}
}

func WithTelemetry() Option {
	return func(c *config) error {
		c.enableTelemetry = true
		return nil
	}
}

func WithIPNI(provider peer.AddrInfo, metadata metadata.Metadata) Option {
	return func(c *config) error {
		mb, err := metadata.MarshalBinary()
		if err != nil {
			return err
		}
		c.ipniConfig = &ipniConfig{
			provider: provider,
			metadata: mb,
		}
		return nil
	}
}

// ListenAndServe creates a new indexing service HTTP server, and starts it up.
func ListenAndServe(addr string, indexer types.Service, opts ...Option) error {
	mux, err := NewServer(indexer, opts...)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	log.Infof("Listening on %s", addr)
	err = srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// NewServer creates a new indexing service HTTP server.
func NewServer(indexer types.Service, opts ...Option) (*http.ServeMux, error) {
	c := &config{}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	if c.id.Issuer == nil {
		log.Warn("Generating a server identity as one has not been set!")
		signer, err := ed25519.Generate()
		if err != nil {
			return nil, fmt.Errorf("generating identity: %w", err)
		}
		c.id = identity.Identity{
			Issuer: multikey.KeyIssuer(signer),
		}
	}

	log.Infof("Server ID: %s", c.id)

	mux := http.NewServeMux()
	maybeInstrumentAndAdd(mux, "GET /", GetRootHandler(c.id), c.enableTelemetry)
	maybeInstrumentAndAdd(mux, "GET /claim/{claim}", GetClaimHandler(indexer), c.enableTelemetry)
	// temporary fix: post claims handler accessible at POST / too
	maybeInstrumentAndAdd(mux, "POST /", PostClaimsHandler(c.id, indexer, c.contentClaimsOptions...), c.enableTelemetry)
	maybeInstrumentAndAdd(mux, "POST /claims", PostClaimsHandler(c.id, indexer, c.contentClaimsOptions...), c.enableTelemetry)
	maybeInstrumentAndAdd(mux, "GET /claims", withGzip(GetClaimsHandler(indexer)), c.enableTelemetry)
	maybeInstrumentAndAdd(mux, "GET /.well-known/did.json", GetDIDDocument(c.id), c.enableTelemetry)
	if c.ipniConfig != nil {
		maybeInstrumentAndAdd(mux, "GET /cid/{cid}", GetIPNICIDHandler(indexer, c.ipniConfig), c.enableTelemetry)
	}
	return mux, nil
}

func maybeInstrumentAndAdd(mux *http.ServeMux, route string, handler http.HandlerFunc, enableTelemetry bool) {
	if enableTelemetry {
		mux.Handle(route, otelhttp.NewHandler(handler, route, otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents)))
	} else {
		mux.HandleFunc(route, handler)
	}
}

// gzipResponseWriter wraps http.ResponseWriter to support gzip compression
type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// withGzip wraps a handler to support gzip compression if the client accepts it
func withGzip(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if client accepts gzip encoding
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			handler(w, r)
			return
		}

		// Set the content encoding header
		w.Header().Set("Content-Encoding", "gzip")

		// Create gzip writer
		gz := gzip.NewWriter(w)
		defer gz.Close()

		// Wrap the response writer
		gzw := gzipResponseWriter{Writer: gz, ResponseWriter: w}
		handler(gzw, r)
	}
}

// GetRootHandler displays version info when a GET request is sent to "/".
func GetRootHandler(id ucan.Issuer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf("🔥 indexing-service %s\n", build.Version)))
		w.Write([]byte("- https://github.com/fil-forge/forge/indexing-service\n"))
		w.Write([]byte(fmt.Sprintf("- %s\n", id)))
	}
}

// GetClaimHandler retrieves a single content claim by it's root CID.
func GetClaimHandler(service types.Getter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		c, err := cid.Parse(parts[len(parts)-1])
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid CID: %s", err), http.StatusBadRequest)
			return
		}

		claim, err := service.Get(r.Context(), c)
		if err != nil {
			if errors.Is(err, types.ErrKeyNotFound) {
				http.Error(w, fmt.Sprintf("not found: %s", c), http.StatusNotFound)
				return
			}
			log.Errorf("getting claim: %s", err)
			http.Error(w, "failed to get claim", http.StatusInternalServerError)
			return
		}

		_, err = w.Write(claim.Bytes())
		if err != nil {
			log.Warnf("serving claim: %s: %s", c, err)
		}
	}
}

// PostClaimsHandler invokes the ucanto service when a POST request is sent to
// "/claims".
func PostClaimsHandler(id ucan.Issuer, service types.Publisher, options ...server.HTTPOption) http.HandlerFunc {
	server, err := contentclaims.NewUCANServer(id, service, options...)
	if err != nil {
		log.Fatalf("creating ucanto server: %s", err)
	}
	return server.ServeHTTP
}

// GetClaimsHandler retrieves content claims when a GET request is sent to
// "/claims?multihash={multihash}".
func GetClaimsHandler(service types.Querier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, s := telemetry.StartSpan(r.Context(), "GetClaimsHandler")
		defer s.End()

		queryTypeParam := r.URL.Query()["type"]
		var queryType types.QueryType
		switch len(queryTypeParam) {
		case 0:
			queryType = types.QueryTypeStandard
		case 1:
			var err error
			queryType, err = types.ParseQueryType(queryTypeParam[0])
			if err != nil {
				http.Error(w, fmt.Sprint(err), http.StatusBadRequest)
				return
			}
		default:
			http.Error(w, fmt.Sprintf("only one 'type' parameter is allowed, but got %d", len(queryTypeParam)), http.StatusBadRequest)
			return
		}

		mhStrings := r.URL.Query()["multihash"]
		hashes := make([]multihash.Multihash, 0, len(mhStrings))
		for _, mhString := range mhStrings {
			_, bytes, err := multibase.Decode(mhString)
			if err != nil {
				http.Error(w, fmt.Sprintf("invalid multibase encoding: %s", err.Error()), http.StatusBadRequest)
				return
			}
			hashes = append(hashes, bytes)
		}
		if len(hashes) == 0 {
			http.Error(w, "missing digests", 400)
			return
		}

		spaceStrings := r.URL.Query()["spaces"]
		spaces := make([]did.DID, 0, len(spaceStrings))
		for _, spaceString := range spaceStrings {
			space, err := did.Parse(spaceString)
			if err != nil {
				http.Error(w, fmt.Sprintf("invalid did: %s", err.Error()), http.StatusBadRequest)
				return
			}
			spaces = append(spaces, space)
		}

		var dlgs []ucan.Delegation
		agentMsgHeader := r.Header.Get(retrieval.HTTPHeaderName)
		if agentMsgHeader != "" {
			msg, err := container.Decode([]byte(agentMsgHeader))
			if err != nil {
				http.Error(w, fmt.Sprintf("decoding agent message: %s", err.Error()), http.StatusBadRequest)
				return
			}
			dlgs = msg.Delegations()
		}

		qr, err := service.Query(ctx, types.Query{
			Type:   queryType,
			Hashes: hashes,
			Match: types.Match{
				Subject: spaces,
			},
			Delegations: dlgs,
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("processing query: %s", err.Error()), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		carWriter := automobile.NewWriter(w)
		err = carWriter.WriteHeader([]cid.Cid{qr.Root().Link})
		if err != nil {
			log.Errorf("writing CAR header: %s", err)
			return
		}
		for _, block := range qr.Blocks() {
			if err := carWriter.WriteBlock(automobile.Block(block)); err != nil {
				log.Errorf("writing CAR block: %s", err)
				return
			}
		}
	}
}

func GetIPNICIDHandler(service types.Querier, config *ipniConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, s := telemetry.StartSpan(r.Context(), "GetClaimsHandler")
		defer s.End()
		if config == nil {
			http.Error(w, "IPNI config not available", http.StatusInternalServerError)
			return
		}
		parts := strings.Split(r.URL.Path, "/")
		c, err := cid.Parse(parts[len(parts)-1])
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid CID: %s", err), http.StatusBadRequest)
			return
		}
		mh := c.Hash()
		qr, err := service.Query(ctx, types.Query{
			Type:   types.QueryTypeStandard,
			Hashes: []multihash.Multihash{mh},
			Match: types.Match{
				Subject: []did.DID{},
			},
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("processing query: %s", err.Error()), http.StatusInternalServerError)
			return
		}
		if len(qr.Claims()) == 0 {
			http.Error(w, fmt.Sprintf("no claims found for CID: %s", c), http.StatusNotFound)
			return
		}

		blocks := map[cid.Cid][]byte{}
		for _, block := range qr.Blocks() {
			blocks[block.Link] = block.Data
		}

		// iterate over all claims to see if there are location claims, return preset peer if found
		for _, root := range qr.Claims() {
			data, ok := blocks[root]
			if !ok {
				http.Error(w, fmt.Sprintf("missing claim: %s:", c), http.StatusInternalServerError)
				continue
			}
			claim, err := invocation.Decode(data)
			if err != nil {
				http.Error(w, fmt.Sprintf("decoding claim: %s", err), http.StatusInternalServerError)
				return
			}

			switch claim.Command() {
			case assertcaps.Location.Command:
				data, err := model.MarshalFindResponse(&model.FindResponse{
					MultihashResults: []model.MultihashResult{{
						Multihash: mh,
						ProviderResults: []model.ProviderResult{{
							ContextID: mh,
							Metadata:  config.metadata,
							Provider:  &config.provider,
						}},
					}},
				})

				if err != nil {
					http.Error(w, fmt.Sprintf("marshalling find response: %s", err), http.StatusInternalServerError)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err = w.Write(data)
				if err != nil {
					log.Errorf("sending find response: %s", err)
				}
				return
			}
		}
		http.Error(w, fmt.Sprintf("no claims found for CID: %s", c), http.StatusNotFound)
	}
}

// GetDIDDocument returns the DID document for did:web resolution.
func GetDIDDocument(issuer multikey.Issuer) http.HandlerFunc {
	id := identity.Identity{Issuer: issuer}
	return func(w http.ResponseWriter, r *http.Request) {
		doc, err := id.DIDDocument()
		if err != nil {
			http.Error(w, fmt.Sprintf("getting DID document: %s", err), http.StatusInternalServerError)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		bytes, err := json.Marshal(doc)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(bytes)
	}
}
