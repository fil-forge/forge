// Package forgeclient is the Forge protocol client for an agent that writes
// to the network through the upload service (sprue): it invokes /blob/add
// (with a deferrable conclude), /ucan/conclude, /blob/abort, /blob/remove,
// /index/add, /provider/add and /access/delegate, runs the /access login
// flow, and polls for the receipts those invocations produce. Proof chains
// come from the client's tokenstore.Store, or per call from any
// ucanlib.ProofStore (WithProofStore).
//
// It descends from github.com/fil-forge/guppy/pkg/client by way of the copy
// ingot carried while it could not import guppy; ingot is its first
// consumer. Relative to that ancestor, blob retrieval is not part of this
// package, the generic Execute is internal/ucanexec's, logging is an
// injected *zap.Logger (WithLogger), and the accept receipt is not required
// to carry a PDP accept invocation.
package forgeclient

import (
	"context"
	"fmt"
	"net/url"

	"github.com/fil-forge/forge/forgeclient/tokenstore"
	"github.com/fil-forge/forge/internal/ucanexec"
	"github.com/fil-forge/forge/protocol/receipt"
	"github.com/fil-forge/ucantone/client"
	"github.com/fil-forge/ucantone/did"
	"github.com/fil-forge/ucantone/execution"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/ipfs/go-cid"
	cbg "github.com/whyrusleeping/cbor-gen"
	"go.uber.org/zap"
)

// Client is a UCAN-over-HTTP client to the upload service (sprue).
type Client struct {
	signer         ucan.Issuer
	serviceID      did.DID
	ucanClient     *client.HTTPClient
	ucanOpts       []client.HTTPOption
	receiptsClient *receipt.Client
	tokenStore     tokenstore.Store
	logger         *zap.Logger
}

// New builds a Client. signer is the agent identity (issuer of every
// invocation); serviceID + serviceURL address sprue.
func New(signer ucan.Issuer, serviceID did.DID, serviceURL url.URL, options ...Option) (*Client, error) {
	c := Client{
		signer:         signer,
		serviceID:      serviceID,
		receiptsClient: receipt.NewClient(serviceURL.JoinPath("/receipt/")),
		logger:         zap.NewNop(),
	}

	for _, opt := range options {
		if err := opt(&c); err != nil {
			return nil, err
		}
	}

	if c.tokenStore == nil {
		c.tokenStore = tokenstore.NewMemStore()
	}

	ucanClient, err := client.NewHTTP(&serviceURL, c.ucanOpts...)
	if err != nil {
		return nil, fmt.Errorf("creating UCAN client: %w", err)
	}
	c.ucanClient = ucanClient

	return &c, nil
}

// DID returns the DID of the agent.
func (c *Client) DID() did.DID { return c.signer.DID() }

// Issuer returns the issuing agent identity.
func (c *Client) Issuer() ucan.Issuer { return c.signer }

// ServiceID returns the DID of the upload service this client targets.
func (c *Client) ServiceID() did.DID { return c.serviceID }

// ProofChain builds a delegation proof chain from aud to sub for cmd.
func (c *Client) ProofChain(ctx context.Context, aud did.DID, cmd ucan.Command, sub did.DID) ([]ucan.Delegation, []cid.Cid, error) {
	return c.tokenStore.ProofChain(ctx, aud, cmd, sub)
}

// AddProofs adds delegations to the client's token store.
func (c *Client) AddProofs(ctx context.Context, delegations ...ucan.Delegation) error {
	return c.tokenStore.AddDelegations(ctx, delegations...)
}

// AddAttestations adds attestations to the client's token store.
func (c *Client) AddAttestations(ctx context.Context, attestations ...ucan.Invocation) error {
	return c.tokenStore.AddInvocations(ctx, attestations...)
}

// Reset clears all tokens from the token store.
func (c *Client) Reset(ctx context.Context) error {
	return c.tokenStore.Reset(ctx)
}

// Execute forwards to internal/ucanexec.Execute. Kept as a package-local
// generic so the ported client code reads unchanged.
func Execute[T cbg.CBORUnmarshaler](
	ctx context.Context,
	executor execution.Executor,
	inv ucan.Invocation,
	options ...execution.RequestOption,
) (T, ucan.Receipt, ucan.Container, error) {
	return ucanexec.Execute[T](ctx, executor, inv, options...)
}
