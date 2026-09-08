package inprocess

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/fil-forge/filecoin-services/go/eip712"
	"github.com/fil-forge/libforge/commands/pdp/sign"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/fil-forge/ucantone/ucan/invocation"

	"github.com/fil-forge/piri-signing-service/pkg/signer"
	"github.com/fil-forge/piri-signing-service/pkg/types"
)

// Signer implements [types.SigningService] using eip712.Signer directly.
// This provides an in-process implementation that bypasses network calls and
// authorization checks — useful for testing and development.
type Signer struct {
	signer *signer.Signer
}

// Verify that Signer implements types.SigningService at compile time
var _ types.SigningService = (*Signer)(nil)

// New creates a new in-process signing service
func New(signer *signer.Signer) *Signer {
	return &Signer{signer: signer}
}

// SignCreateDataSet signs a CreateDataSet operation directly.
func (s *Signer) SignCreateDataSet(_ context.Context,
	_ ucan.Issuer,
	clientDataSetId *big.Int,
	payee common.Address,
	metadata []eip712.MetadataEntry,
	_ []ucan.Delegation,
	_ ...invocation.Option) (*eip712.AuthSignature, error) {
	return s.signer.SignCreateDataSet(clientDataSetId, payee, metadata)
}

// SignAddPieces signs an AddPieces operation directly.
func (s *Signer) SignAddPieces(_ context.Context,
	_ ucan.Issuer,
	clientDataSetId *big.Int,
	nonce *big.Int,
	pieceData [][]byte,
	metadata [][]eip712.MetadataEntry,
	_ []sign.PieceProofs,
	_ ucan.Container,
	_ []ucan.Delegation,
	_ ...invocation.Option) (*eip712.AuthSignature, error) {
	return s.signer.SignAddPieces(clientDataSetId, nonce, pieceData, metadata)
}

// SignSchedulePieceRemovals signs a SchedulePieceRemovals operation directly.
func (s *Signer) SignSchedulePieceRemovals(_ context.Context,
	_ ucan.Issuer,
	clientDataSetId *big.Int,
	pieceIds []*big.Int,
	_ []ucan.Delegation,
	_ ...invocation.Option) (*eip712.AuthSignature, error) {
	return s.signer.SignSchedulePieceRemovals(clientDataSetId, pieceIds)
}

// SignDeleteDataSet signs a DeleteDataSet operation directly.
func (s *Signer) SignDeleteDataSet(_ context.Context,
	_ ucan.Issuer,
	clientDataSetId *big.Int,
	_ []ucan.Delegation,
	_ ...invocation.Option) (*eip712.AuthSignature, error) {
	return s.signer.SignDeleteDataSet(clientDataSetId)
}
