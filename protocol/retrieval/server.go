package retrieval

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/fil-forge/ucantone/execution"
	"github.com/fil-forge/ucantone/server"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/fil-forge/ucantone/ucan/container"
)

// Server is a UCAN server that accepts a single executable invocation
// carried in the "X-UCAN-Container" HTTP header, leaving the request and
// response bodies free to transport raw bytes for the requested blob.
//
// The original HTTP request is exposed to handlers as a
// [*HTTPHeaderRequestContainer] on the execution request metadata, and a
// handler may set a [*HTTPHeaderResponseContainer] as its response metadata
// to control the HTTP response status code, headers and body.
//
// Since the request and response bodies can only service a single retrieval,
// the request container MUST contain exactly one invocation addressed to
// this server (additional invocations not addressed to the server are
// permitted and ignored).
type Server struct {
	*server.HTTPServer
	id    ucan.Issuer
	codec *HTTPHeaderInboundCodec
}

// NewServer creates a new UCAN retrieval server.
func NewServer(id ucan.Issuer, options ...server.HTTPOption) *Server {
	codec := DefaultHTTPHeaderInboundCodec
	options = append(options, server.WithHTTPCodec(codec))
	return &Server{
		HTTPServer: server.NewHTTP(id, options...),
		id:         id,
		codec:      codec,
	}
}

// ServeHTTP implements [http.Handler].
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resp, err := s.RoundTrip(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("handling request: %v", err), http.StatusInternalServerError)
		return
	}
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	if resp.Body != nil {
		io.Copy(w, resp.Body)
		resp.Body.Close()
	}
}

// RoundTrip unpacks and executes an incoming request, returning the response.
func (s *Server) RoundTrip(r *http.Request) (*http.Response, error) {
	reqContainer, err := s.codec.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("decoding request: %w", err)
	}

	var inv ucan.Invocation
	for _, candidate := range reqContainer.Invocations() {
		aud := candidate.Audience()
		if !aud.Defined() {
			aud = candidate.Subject()
		}
		if aud != s.id.DID() {
			continue
		}
		if inv != nil {
			return nil, fmt.Errorf("expected exactly 1 invocation addressed to the server, found multiple")
		}
		inv = candidate
	}
	if inv == nil {
		return nil, fmt.Errorf("missing UCAN invocation")
	}

	res, err := s.Execute(&request{r.Context(), inv, reqContainer})
	if err != nil {
		return nil, fmt.Errorf("executing task %s: %w", inv.Task().Link(), err)
	}

	receipts := []ucan.Receipt{res.Receipt()}
	var invocations []ucan.Invocation
	var delegations []ucan.Delegation
	var httpMeta *HTTPHeaderResponseContainer
	if res.Metadata() != nil {
		invocations = append(invocations, res.Metadata().Invocations()...)
		delegations = append(delegations, res.Metadata().Delegations()...)
		receipts = append(receipts, res.Metadata().Receipts()...)
		httpMeta, _ = res.Metadata().(*HTTPHeaderResponseContainer)
	}

	respContainer := container.New(
		container.WithInvocations(invocations...),
		container.WithDelegations(delegations...),
		container.WithReceipts(receipts...),
	)

	if httpMeta != nil {
		return s.codec.Encode(&HTTPHeaderResponseContainer{
			Container:  respContainer,
			StatusCode: httpMeta.StatusCode,
			Header:     httpMeta.Header,
			Body:       httpMeta.Body,
		})
	}

	return s.codec.Encode(respContainer)
}

// request implements [execution.Request] and lets us pass the
// [HTTPHeaderRequestContainer] through to handlers as request metadata.
type request struct {
	ctx        context.Context
	invocation ucan.Invocation
	metadata   ucan.Container
}

var _ execution.Request = (*request)(nil)

func (r *request) Context() context.Context    { return r.ctx }
func (r *request) Invocation() ucan.Invocation { return r.invocation }
func (r *request) Metadata() ucan.Container    { return r.metadata }
