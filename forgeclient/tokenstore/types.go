// Package tokenstore persists the UCAN tokens an agent holds — the
// delegations that make up its proof chains, and the invocations and
// receipts it has issued or received — and serves them as a
// ucanlib.ProofStore. FsStore keeps them in a CBOR file, MemStore in memory.
//
// It descends from github.com/fil-forge/guppy/pkg/tokenstore.
package tokenstore

import (
	"context"
	"iter"

	"github.com/fil-forge/ucantone/did"
	"github.com/fil-forge/ucantone/ucan"
	ucanlib "github.com/fil-forge/ucantone/ucanlib"
)

// Store is a ucanlib.ProofStore that additionally accepts new
// delegations/attestations/receipts and supports enumeration + reset.
type Store interface {
	ucanlib.ProofStore
	// AddInvocations adds the given invocations to the store.
	AddInvocations(ctx context.Context, invocations ...ucan.Invocation) error
	// AddDelegations adds the given delegations to the store.
	AddDelegations(ctx context.Context, delegations ...ucan.Delegation) error
	// AddReceipts adds the given receipts to the store.
	AddReceipts(ctx context.Context, receipts ...ucan.Receipt) error
	// ListDelegations returns a sequence of delegations matching the given criteria.
	ListDelegations(ctx context.Context, aud did.DID, cmd ucan.Command, sub did.DID) iter.Seq2[ucan.Delegation, error]
	// Delegations returns all delegations held by the store.
	Delegations(ctx context.Context) ([]ucan.Delegation, error)
	// Reset clears all data in the store.
	Reset(ctx context.Context) error
}
