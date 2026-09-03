package hilt

import (
	"net/http"

	ucanlib "github.com/fil-forge/libforge/ucan"
	"github.com/fil-forge/ucantone/ucan"
	"go.uber.org/zap"
)

// Option configures a [Client] at construction.
type Option func(*clientConfig)

type clientConfig struct {
	httpClient *http.Client
	logger     *zap.Logger
}

// WithHTTPClient sets the *http.Client the UCAN executor sends requests with.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(cfg *clientConfig) {
		cfg.httpClient = httpClient
	}
}

// WithLogger sets the client's logger; a nil logger is ignored.
func WithLogger(logger *zap.Logger) Option {
	return func(cfg *clientConfig) {
		if logger != nil {
			cfg.logger = logger
		}
	}
}

// MethodOption overrides a [Client]'s defaults for a single call.
type MethodOption func(*methodConfig)

type methodConfig struct {
	issuer ucan.Issuer
	proofs ucanlib.ProofStore
}

// WithIssuer overrides the invocation issuer for one call; nil is ignored.
func WithIssuer(iss ucan.Issuer) MethodOption {
	return func(cfg *methodConfig) {
		if iss != nil {
			cfg.issuer = iss
		}
	}
}

// WithProofs overrides the proof store for one call; nil is ignored.
func WithProofs(proofs ucanlib.ProofStore) MethodOption {
	return func(cfg *methodConfig) {
		if proofs != nil {
			cfg.proofs = proofs
		}
	}
}
