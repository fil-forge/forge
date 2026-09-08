package inprocess

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fil-forge/filecoin-services/go/eip712"
	"github.com/fil-forge/ucantone/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fil-forge/piri-signing-service/pkg/signer"
	"github.com/fil-forge/piri-signing-service/pkg/types"
)

func setupTestSigner(t *testing.T) (*Signer, *ecdsa.PrivateKey) {
	// Generate a test private key
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	// Create a test contract address
	contractAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// Create a test chain ID
	chainID := big.NewInt(31415926)

	// Create the signer
	eip712Signer := signer.NewSigner(privateKey, chainID, contractAddr)

	// Create the in-process signer
	signer := New(eip712Signer)

	return signer, privateKey
}

func TestSigner_ImplementsInterface(t *testing.T) {
	signer, _ := setupTestSigner(t)
	var _ types.SigningService = signer
}

func TestSigner_SignCreateDataSet(t *testing.T) {
	s, privateKey := setupTestSigner(t)
	issuer := testutil.RandomIssuer(t)
	ctx := context.Background()

	clientDataSetId := big.NewInt(12345)
	payee := common.HexToAddress("0xabcdef1234567890abcdef1234567890abcdef12")
	metadata := []eip712.MetadataEntry{
		{Key: "test-key", Value: "test-value"},
	}

	signature, err := s.SignCreateDataSet(ctx, issuer, clientDataSetId, payee, metadata, nil)
	require.NoError(t, err)
	assert.NotNil(t, signature)

	expectedAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	assert.Equal(t, expectedAddress, signature.Signer)

	assert.NotEmpty(t, signature.R)
	assert.NotEmpty(t, signature.S)
	assert.NotZero(t, signature.V)
}

func TestSigner_SignAddPieces(t *testing.T) {
	s, privateKey := setupTestSigner(t)
	issuer := testutil.RandomIssuer(t)
	ctx := context.Background()

	clientDataSetId := big.NewInt(12345)
	firstAdded := big.NewInt(0)
	pieceData := [][]byte{
		[]byte("piece1"),
		[]byte("piece2"),
	}
	metadata := [][]eip712.MetadataEntry{
		{{Key: "piece1-key", Value: "piece1-value"}},
		{{Key: "piece2-key", Value: "piece2-value"}},
	}

	signature, err := s.SignAddPieces(ctx, issuer, clientDataSetId, firstAdded, pieceData, metadata, nil, nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, signature)

	expectedAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	assert.Equal(t, expectedAddress, signature.Signer)

	assert.NotEmpty(t, signature.R)
	assert.NotEmpty(t, signature.S)
	assert.NotZero(t, signature.V)
}

func TestSigner_SignSchedulePieceRemovals(t *testing.T) {
	s, privateKey := setupTestSigner(t)
	issuer := testutil.RandomIssuer(t)
	ctx := context.Background()

	clientDataSetId := big.NewInt(12345)
	pieceIds := []*big.Int{
		big.NewInt(1),
		big.NewInt(2),
		big.NewInt(3),
	}

	signature, err := s.SignSchedulePieceRemovals(ctx, issuer, clientDataSetId, pieceIds, nil)
	require.NoError(t, err)
	assert.NotNil(t, signature)

	expectedAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	assert.Equal(t, expectedAddress, signature.Signer)

	assert.NotEmpty(t, signature.R)
	assert.NotEmpty(t, signature.S)
	assert.NotZero(t, signature.V)
}

func TestSigner_SignDeleteDataSet(t *testing.T) {
	s, privateKey := setupTestSigner(t)
	issuer := testutil.RandomIssuer(t)
	ctx := context.Background()

	clientDataSetId := big.NewInt(12345)

	signature, err := s.SignDeleteDataSet(ctx, issuer, clientDataSetId, nil)
	require.NoError(t, err)
	assert.NotNil(t, signature)

	expectedAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	assert.Equal(t, expectedAddress, signature.Signer)

	assert.NotEmpty(t, signature.R)
	assert.NotEmpty(t, signature.S)
	assert.NotZero(t, signature.V)
}

func TestSigner_SignatureConsistency(t *testing.T) {
	s, _ := setupTestSigner(t)
	issuer := testutil.RandomIssuer(t)
	ctx := context.Background()

	clientDataSetId := big.NewInt(12345)
	payee := common.HexToAddress("0xabcdef1234567890abcdef1234567890abcdef12")
	metadata := []eip712.MetadataEntry{
		{Key: "test-key", Value: "test-value"},
	}

	sig1, err := s.SignCreateDataSet(ctx, issuer, clientDataSetId, payee, metadata, nil)
	require.NoError(t, err)

	sig2, err := s.SignCreateDataSet(ctx, issuer, clientDataSetId, payee, metadata, nil)
	require.NoError(t, err)

	assert.Equal(t, sig1.R, sig2.R)
	assert.Equal(t, sig1.S, sig2.S)
	assert.Equal(t, sig1.V, sig2.V)
	assert.Equal(t, sig1.Signer, sig2.Signer)
}
