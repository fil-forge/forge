package client

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"net/http"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
	"github.com/fil-forge/filecoin-services/go/eip712"
	"github.com/fil-forge/libforge/commands/pdp/sign"
	"github.com/fil-forge/ucantone/client"
	"github.com/fil-forge/ucantone/did"
	"github.com/fil-forge/ucantone/errors"
	"github.com/fil-forge/ucantone/execution"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/fil-forge/ucantone/ucan/invocation"
	"github.com/ipfs/go-cid"

	"github.com/fil-forge/piri-signing-service/pkg/types"
)

// Client uses UCAN invocations to request a remote signing service to sign PDP operations.
type Client struct {
	ServiceDID        did.DID             // DID of the remote signing service.
	HTTP              *client.HTTPClient  // ucantone HTTP client.
	InvocationOptions []invocation.Option // options added to every invocation.
}

// Verify that Client implements types.SigningService at compile time
var _ types.SigningService = (*Client)(nil)

type clientConfig struct {
	httpClient        *http.Client
	invocationOptions []invocation.Option
}

type Option func(*clientConfig)

// WithHTTPClient configures a custom HTTP client.
func WithHTTPClient(c *http.Client) Option {
	return func(cfg *clientConfig) {
		cfg.httpClient = c
	}
}

// WithInvocationOptions configures invocation options that are sent with every
// request.
func WithInvocationOptions(opts ...invocation.Option) Option {
	return func(cfg *clientConfig) {
		cfg.invocationOptions = append(cfg.invocationOptions, opts...)
	}
}

// New creates a new client for the signing service.
func New(serviceDID did.DID, serviceURL string, options ...Option) (*Client, error) {
	cfg := clientConfig{}
	for _, opt := range options {
		opt(&cfg)
	}
	endpoint, err := url.Parse(serviceURL)
	if err != nil {
		return nil, fmt.Errorf("parsing signing service URL: %w", err)
	}
	var httpOpts []client.HTTPOption
	if cfg.httpClient != nil {
		httpOpts = append(httpOpts, client.WithHTTPClient(cfg.httpClient))
	}
	httpClient, err := client.NewHTTP(endpoint, httpOpts...)
	if err != nil {
		return nil, fmt.Errorf("creating signing service HTTP client: %w", err)
	}
	return &Client{
		ServiceDID:        serviceDID,
		HTTP:              httpClient,
		InvocationOptions: cfg.invocationOptions,
	}, nil
}

// SignCreateDataSet signs a CreateDataSet operation via UCAN invocation.
func (c *Client) SignCreateDataSet(
	ctx context.Context,
	issuer ucan.Issuer,
	dataSet *big.Int,
	payee common.Address,
	metadata []eip712.MetadataEntry,
	proofs []ucan.Delegation,
	options ...invocation.Option,
) (*eip712.AuthSignature, error) {
	args := &sign.DataSetCreateArguments{
		DataSet:  dataSet,
		Payee:    payee.Bytes(),
		Metadata: fromEIP712MetadataEntries(metadata),
	}
	inv, err := sign.DataSetCreate.Invoke(issuer, c.ServiceDID, args, c.mergeOptionsWithProofs(options, proofs)...)
	if err != nil {
		return nil, fmt.Errorf("invoking %s: %w", sign.DataSetCreate.Command, err)
	}
	return c.execAuthSignatureRequest(ctx, inv, execution.WithDelegations(proofs...))
}

// SignAddPieces signs an AddPieces operation via UCAN invocation. The
// caller MUST supply a proofContainer that includes every blob/accept
// invocation referenced by pieceProofs[].Proofs, plus their pdp/accept
// receipts.
func (c *Client) SignAddPieces(
	ctx context.Context,
	issuer ucan.Issuer,
	dataSet *big.Int,
	nonce *big.Int,
	pieceData [][]byte,
	metadata [][]eip712.MetadataEntry,
	pieceProofs []sign.PieceProofs,
	proofContainer ucan.Container,
	proofs []ucan.Delegation,
	options ...invocation.Option,
) (*eip712.AuthSignature, error) {
	meta := make([]sign.Metadata, 0, len(metadata))
	for _, m := range metadata {
		meta = append(meta, fromEIP712MetadataEntries(m))
	}
	args := &sign.PiecesAddArguments{
		DataSet:   dataSet,
		Nonce:     nonce,
		PieceData: pieceData,
		Metadata:  meta,
		Proofs:    pieceProofs,
	}
	inv, err := sign.PiecesAdd.Invoke(issuer, c.ServiceDID, args, c.mergeOptionsWithProofs(options, proofs)...)
	if err != nil {
		return nil, fmt.Errorf("invoking %s: %w", sign.PiecesAdd.Command, err)
	}

	reqOpts := []execution.RequestOption{execution.WithDelegations(proofs...)}
	if proofContainer != nil {
		reqOpts = append(reqOpts,
			execution.WithInvocations(proofContainer.Invocations()...),
			execution.WithReceipts(proofContainer.Receipts()...),
			execution.WithDelegations(proofContainer.Delegations()...),
		)
	}
	return c.execAuthSignatureRequest(ctx, inv, reqOpts...)
}

// SignSchedulePieceRemovals signs a SchedulePieceRemovals operation via UCAN invocation.
func (c *Client) SignSchedulePieceRemovals(
	ctx context.Context,
	issuer ucan.Issuer,
	dataSet *big.Int,
	pieceIds []*big.Int,
	proofs []ucan.Delegation,
	options ...invocation.Option,
) (*eip712.AuthSignature, error) {
	args := &sign.PiecesRemoveScheduleArguments{
		DataSet: dataSet,
		Pieces:  pieceIds,
	}
	inv, err := sign.PiecesRemoveSchedule.Invoke(issuer, c.ServiceDID, args, c.mergeOptionsWithProofs(options, proofs)...)
	if err != nil {
		return nil, fmt.Errorf("invoking %s: %w", sign.PiecesRemoveSchedule.Command, err)
	}
	return c.execAuthSignatureRequest(ctx, inv, execution.WithDelegations(proofs...))
}

// SignDeleteDataSet signs a DeleteDataSet operation via UCAN invocation.
func (c *Client) SignDeleteDataSet(
	ctx context.Context,
	issuer ucan.Issuer,
	dataSet *big.Int,
	proofs []ucan.Delegation,
	options ...invocation.Option,
) (*eip712.AuthSignature, error) {
	args := &sign.DataSetDeleteArguments{DataSet: dataSet}
	inv, err := sign.DataSetDelete.Invoke(issuer, c.ServiceDID, args, c.mergeOptionsWithProofs(options, proofs)...)
	if err != nil {
		return nil, fmt.Errorf("invoking %s: %w", sign.DataSetDelete.Command, err)
	}
	return c.execAuthSignatureRequest(ctx, inv, execution.WithDelegations(proofs...))
}

func (c *Client) mergeOptionsWithProofs(options []invocation.Option, proofs []ucan.Delegation) []invocation.Option {
	all := make([]invocation.Option, 0, len(c.InvocationOptions)+len(options)+1)
	all = append(all, c.InvocationOptions...)
	all = append(all, options...)
	if len(proofs) > 0 {
		links := make([]cid.Cid, len(proofs))
		for i, p := range proofs {
			links[i] = p.Link()
		}
		all = append(all, invocation.WithProofs(links...))
	}
	return all
}

func (c *Client) execAuthSignatureRequest(ctx context.Context, inv ucan.Invocation, reqOpts ...execution.RequestOption) (*eip712.AuthSignature, error) {
	req := execution.NewRequest(ctx, inv, reqOpts...)
	resp, err := c.HTTP.Execute(req)
	if err != nil {
		return nil, fmt.Errorf("executing %s: %w", inv.Command(), err)
	}

	rcpt := resp.Receipt()
	out := rcpt.Out()
	okBytes, errBytes := out.Unpack()
	if !out.IsOK() {
		// Try to decode as a named error; fall back to a generic message.
		named := errors.Named(nil)
		if e := tryDecodeNamedError(errBytes); e != nil {
			named = e
		}
		if named != nil {
			return nil, named
		}
		return nil, fmt.Errorf("%s failed: %s", inv.Command(), string(errBytes))
	}

	var sig sign.AuthSignature
	if err := sig.UnmarshalCBOR(bytes.NewReader(okBytes)); err != nil {
		return nil, fmt.Errorf("decoding signature receipt: %w", err)
	}
	out712, err := fromLibforgeAuthSig(sig)
	if err != nil {
		return nil, fmt.Errorf("converting signature: %w", err)
	}
	return out712, nil
}

// tryDecodeNamedError attempts to decode raw failure bytes as the ucantone
// error datamodel; returns nil if the bytes don't match.
func tryDecodeNamedError(b []byte) errors.Named {
	// errors.New returns a value implementing the Named interface and
	// MarshalCBOR-able as a tagged error model. For now we don't decode it
	// back into a typed value — callers get the raw bytes via the fallback.
	// Hook left in place for future structured failure parsing.
	_ = b
	return nil
}

func fromEIP712MetadataEntries(entries []eip712.MetadataEntry) sign.Metadata {
	meta := sign.Metadata{
		Keys:   make([]string, 0, len(entries)),
		Values: make(map[string]string, len(entries)),
	}
	for _, e := range entries {
		meta.Keys = append(meta.Keys, e.Key)
		meta.Values[e.Key] = e.Value
	}
	return meta
}

func fromLibforgeAuthSig(s sign.AuthSignature) (*eip712.AuthSignature, error) {
	if len(s.R) != len(common.Hash{}) {
		return nil, fmt.Errorf("invalid R length: got %d, want %d", len(s.R), len(common.Hash{}))
	}
	if len(s.S) != len(common.Hash{}) {
		return nil, fmt.Errorf("invalid S length: got %d, want %d", len(s.S), len(common.Hash{}))
	}
	if len(s.Signer) != len(common.Address{}) {
		return nil, fmt.Errorf("invalid Signer length: got %d, want %d", len(s.Signer), len(common.Address{}))
	}
	out := &eip712.AuthSignature{
		Signature:  s.Signature,
		V:          s.V,
		SignedData: s.SignedData,
	}
	copy(out.R[:], s.R)
	copy(out.S[:], s.S)
	out.Signer.SetBytes(s.Signer)
	return out, nil
}
