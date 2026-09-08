package handlers_test

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fil-forge/libforge/commands/pdp/sign"
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fil-forge/piri-signing-service/pkg/server/handlers"
	"github.com/fil-forge/piri-signing-service/pkg/signer"
)

// createTestSigner creates a test eip712 signer with a random key.
func createTestSigner(t *testing.T) *signer.Signer {
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	chainID := big.NewInt(314159) // Calibration testnet
	contractAddr := common.HexToAddress("0x8b7aa0a68f5717e400F1C4D37F7a28f84f76dF91")

	return signer.NewSigner(privateKey, chainID, contractAddr)
}

func TestSignCreateDataSet_Success(t *testing.T) {
	alice := testutil.RandomIssuer(t)
	service := testutil.RandomIssuer(t)

	s := createTestSigner(t)
	handler := handlers.NewDataSetCreateHandler(service, s)

	testPayee := common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb")

	args := &sign.DataSetCreateArguments{
		DataSet: big.NewInt(123),
		Payee:   testPayee.Bytes(),
		Metadata: sign.Metadata{
			Keys:   []string{"name", "version"},
			Values: map[string]string{"name": "test-dataset", "version": "1.0"},
		},
	}
	inv, err := sign.DataSetCreate.Invoke(alice, service.DID(), args)
	require.NoError(t, err)

	req, err := binding.NewRequest[*sign.DataSetCreateArguments](t.Context(), inv)
	require.NoError(t, err)
	res, err := binding.NewResponse(inv.Task().Link(), binding.WithIssuer[*sign.DataSetCreateOK](service))
	require.NoError(t, err)

	require.NoError(t, handler(req, res))
	okBytes, errBytes := res.Receipt().Out().Unpack()
	require.Nil(t, errBytes)
	require.NotNil(t, okBytes)

	var got sign.AuthSignature
	require.NoError(t, got.UnmarshalCBOR(bytes.NewReader(okBytes)))
	assertSignature(t, s.GetAddress(), got)
}

func TestSignCreateDataSet_InvalidResource(t *testing.T) {
	alice := testutil.RandomIssuer(t)
	service := testutil.RandomIssuer(t)

	s := createTestSigner(t)
	handler := handlers.NewDataSetCreateHandler(service, s)

	testPayee := common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb")

	args := &sign.DataSetCreateArguments{
		DataSet: big.NewInt(123),
		Payee:   testPayee.Bytes(),
		Metadata: sign.Metadata{
			Keys:   []string{"name", "version"},
			Values: map[string]string{"name": "test-dataset", "version": "1.0"},
		},
	}
	// invoke with alice's DID as the subject (instead of the service DID)
	inv, err := sign.DataSetCreate.Invoke(alice, alice.DID(), args)
	require.NoError(t, err)

	req, err := binding.NewRequest[*sign.DataSetCreateArguments](t.Context(), inv)
	require.NoError(t, err)
	res, err := binding.NewResponse(inv.Task().Link(), binding.WithIssuer[*sign.DataSetCreateOK](service))
	require.NoError(t, err)

	require.NoError(t, handler(req, res))
	require.False(t, res.Receipt().Out().IsOK())
}

func TestSignDeleteDataSet_Success(t *testing.T) {
	alice := testutil.RandomIssuer(t)
	service := testutil.RandomIssuer(t)

	s := createTestSigner(t)
	handler := handlers.NewDataSetDeleteHandler(service, s)

	args := &sign.DataSetDeleteArguments{DataSet: big.NewInt(123)}
	inv, err := sign.DataSetDelete.Invoke(alice, service.DID(), args)
	require.NoError(t, err)

	req, err := binding.NewRequest[*sign.DataSetDeleteArguments](t.Context(), inv)
	require.NoError(t, err)
	res, err := binding.NewResponse(inv.Task().Link(), binding.WithIssuer[*sign.DataSetDeleteOK](service))
	require.NoError(t, err)

	require.NoError(t, handler(req, res))
	okBytes, errBytes := res.Receipt().Out().Unpack()
	require.Nil(t, errBytes)
	require.NotNil(t, okBytes)

	var got sign.AuthSignature
	require.NoError(t, got.UnmarshalCBOR(bytes.NewReader(okBytes)))
	assertSignature(t, s.GetAddress(), got)
}

func TestSignDeleteDataSet_InvalidResource(t *testing.T) {
	alice := testutil.RandomIssuer(t)
	service := testutil.RandomIssuer(t)

	s := createTestSigner(t)
	handler := handlers.NewDataSetDeleteHandler(service, s)

	args := &sign.DataSetDeleteArguments{DataSet: big.NewInt(123)}
	inv, err := sign.DataSetDelete.Invoke(alice, alice.DID(), args)
	require.NoError(t, err)

	req, err := binding.NewRequest[*sign.DataSetDeleteArguments](t.Context(), inv)
	require.NoError(t, err)
	res, err := binding.NewResponse(inv.Task().Link(), binding.WithIssuer[*sign.DataSetDeleteOK](service))
	require.NoError(t, err)

	require.NoError(t, handler(req, res))
	require.False(t, res.Receipt().Out().IsOK())
}

func TestSignAddPieces_Success(t *testing.T) {
	alice := testutil.RandomIssuer(t)
	service := testutil.RandomIssuer(t)

	s := createTestSigner(t)
	handler := handlers.NewPiecesAddHandler(service, s)

	args := &sign.PiecesAddArguments{
		DataSet: big.NewInt(123),
		Nonce:   big.NewInt(0),
		PieceData: [][]byte{
			mustHex(t, "0001020304"),
			mustHex(t, "0506070809"),
		},
		Metadata: []sign.Metadata{
			{Keys: []string{"size"}, Values: map[string]string{"size": "1024"}},
			{Keys: []string{"size"}, Values: map[string]string{"size": "2048"}},
		},
	}
	inv, err := sign.PiecesAdd.Invoke(alice, service.DID(), args)
	require.NoError(t, err)

	req, err := binding.NewRequest[*sign.PiecesAddArguments](t.Context(), inv)
	require.NoError(t, err)
	res, err := binding.NewResponse(inv.Task().Link(), binding.WithIssuer[*sign.PiecesAddOK](service))
	require.NoError(t, err)

	require.NoError(t, handler(req, res))
	okBytes, errBytes := res.Receipt().Out().Unpack()
	require.Nil(t, errBytes)
	require.NotNil(t, okBytes)

	var got sign.AuthSignature
	require.NoError(t, got.UnmarshalCBOR(bytes.NewReader(okBytes)))
	assertSignature(t, s.GetAddress(), got)
}

func TestSignAddPieces_InvalidResource(t *testing.T) {
	alice := testutil.RandomIssuer(t)
	service := testutil.RandomIssuer(t)

	s := createTestSigner(t)
	handler := handlers.NewPiecesAddHandler(service, s)

	args := &sign.PiecesAddArguments{
		DataSet: big.NewInt(123),
		Nonce:   big.NewInt(0),
	}
	inv, err := sign.PiecesAdd.Invoke(alice, alice.DID(), args)
	require.NoError(t, err)

	req, err := binding.NewRequest[*sign.PiecesAddArguments](t.Context(), inv)
	require.NoError(t, err)
	res, err := binding.NewResponse(inv.Task().Link(), binding.WithIssuer[*sign.PiecesAddOK](service))
	require.NoError(t, err)

	require.NoError(t, handler(req, res))
	require.False(t, res.Receipt().Out().IsOK())
}

func TestSignScheduleRemovePieces_Success(t *testing.T) {
	alice := testutil.RandomIssuer(t)
	service := testutil.RandomIssuer(t)

	s := createTestSigner(t)
	handler := handlers.NewPiecesRemoveScheduleHandler(service, s)

	args := &sign.PiecesRemoveScheduleArguments{
		DataSet: big.NewInt(123),
		Pieces:  []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)},
	}
	inv, err := sign.PiecesRemoveSchedule.Invoke(alice, service.DID(), args)
	require.NoError(t, err)

	req, err := binding.NewRequest[*sign.PiecesRemoveScheduleArguments](t.Context(), inv)
	require.NoError(t, err)
	res, err := binding.NewResponse(inv.Task().Link(), binding.WithIssuer[*sign.PiecesRemoveScheduleOK](service))
	require.NoError(t, err)

	require.NoError(t, handler(req, res))
	okBytes, errBytes := res.Receipt().Out().Unpack()
	require.Nil(t, errBytes)
	require.NotNil(t, okBytes)

	var got sign.AuthSignature
	require.NoError(t, got.UnmarshalCBOR(bytes.NewReader(okBytes)))
	assertSignature(t, s.GetAddress(), got)
}

func TestSignScheduleRemovePieces_InvalidResource(t *testing.T) {
	alice := testutil.RandomIssuer(t)
	service := testutil.RandomIssuer(t)

	s := createTestSigner(t)
	handler := handlers.NewPiecesRemoveScheduleHandler(service, s)

	args := &sign.PiecesRemoveScheduleArguments{
		DataSet: big.NewInt(123),
		Pieces:  []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)},
	}
	inv, err := sign.PiecesRemoveSchedule.Invoke(alice, alice.DID(), args)
	require.NoError(t, err)

	req, err := binding.NewRequest[*sign.PiecesRemoveScheduleArguments](t.Context(), inv)
	require.NoError(t, err)
	res, err := binding.NewResponse(inv.Task().Link(), binding.WithIssuer[*sign.PiecesRemoveScheduleOK](service))
	require.NoError(t, err)

	require.NoError(t, handler(req, res))
	require.False(t, res.Receipt().Out().IsOK())
}

func assertSignature(t *testing.T, signerAddr common.Address, sig sign.AuthSignature) {
	assert.NotEmpty(t, sig.Signature)
	require.Len(t, sig.Signer, len(common.Address{}))
	assert.Equal(t, signerAddr, common.BytesToAddress(sig.Signer))
	assert.NotEmpty(t, sig.SignedData)
	assert.True(t, sig.V == 27 || sig.V == 28, "V should be 27 or 28")
}

func mustHex(t *testing.T, s string) []byte {
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	return b
}
