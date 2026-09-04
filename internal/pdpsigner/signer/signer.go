package signer

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fil-forge/filecoin-services/go/eip712"
)

type Signer struct {
	privateKey        *ecdsa.PrivateKey
	address           common.Address
	chainId           *big.Int
	verifyingContract common.Address
}

func NewSigner(privateKey *ecdsa.PrivateKey, chainId *big.Int, verifyingContract common.Address) *Signer {
	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	return &Signer{
		privateKey:        privateKey,
		address:           address,
		chainId:           chainId,
		verifyingContract: verifyingContract,
	}
}

func (s *Signer) GetAddress() common.Address {
	return s.address
}

func (s *Signer) SignCreateDataSet(clientDataSetId *big.Int, payee common.Address, metadata []eip712.MetadataEntry) (*eip712.AuthSignature, error) {
	// Convert metadata to the format expected by apitypes
	metadataArray := make([]map[string]interface{}, len(metadata))
	for i, entry := range metadata {
		metadataArray[i] = map[string]interface{}{
			"key":   entry.Key,
			"value": entry.Value,
		}
	}

	message := map[string]interface{}{
		"clientDataSetId": clientDataSetId,
		"payee":           strings.ToLower(payee.Hex()),
		"metadata":        metadataArray,
	}

	return s.signTypedData("CreateDataSet", message)
}

// RecoverCreateDataSetSigner recovers the signer address from a CreateDataSet signature
func (s *Signer) RecoverCreateDataSetSigner(clientDataSetId *big.Int, payee common.Address, metadata []eip712.MetadataEntry, signature *eip712.AuthSignature) (common.Address, error) {
	// Convert metadata to the format expected by apitypes
	metadataArray := make([]map[string]interface{}, len(metadata))
	for i, entry := range metadata {
		metadataArray[i] = map[string]interface{}{
			"key":   entry.Key,
			"value": entry.Value,
		}
	}

	message := map[string]interface{}{
		"clientDataSetId": clientDataSetId,
		"payee":           strings.ToLower(payee.Hex()),
		"metadata":        metadataArray,
	}

	domain := eip712.GetDomain(s.chainId, s.verifyingContract)
	hash, err := eip712.GetMessageHash(domain, "CreateDataSet", message)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to get message hash: %w", err)
	}

	// crypto.SigToPub expects v to be 0 or 1, but signature has v as 27 or 28
	// Create a copy with adjusted v for recovery
	recoverySignature := make([]byte, 65)
	copy(recoverySignature, signature.Signature)
	if recoverySignature[64] >= 27 {
		recoverySignature[64] -= 27
	}

	// Recover the public key from the signature
	pubKey, err := crypto.SigToPub(hash, recoverySignature)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to recover public key: %w", err)
	}

	return crypto.PubkeyToAddress(*pubKey), nil
}

func (s *Signer) SignAddPieces(clientDataSetId, nonce *big.Int, pieceData [][]byte, metadata [][]eip712.MetadataEntry) (*eip712.AuthSignature, error) {
	cids := make([]map[string]interface{}, len(pieceData))
	for i, data := range pieceData {
		cids[i] = map[string]interface{}{
			"data": data,
		}
	}

	pieceMetadata := make([]map[string]interface{}, len(metadata))
	for i, meta := range metadata {
		// Convert MetadataEntry array to expected format
		metadataArray := make([]map[string]interface{}, len(meta))
		for j, entry := range meta {
			metadataArray[j] = map[string]interface{}{
				"key":   entry.Key,
				"value": entry.Value,
			}
		}

		pieceMetadata[i] = map[string]interface{}{
			"pieceIndex": big.NewInt(int64(i)),
			"metadata":   metadataArray,
		}
	}

	message := map[string]interface{}{
		"clientDataSetId": clientDataSetId,
		"nonce":           nonce,
		"pieceData":       cids,
		"pieceMetadata":   pieceMetadata,
	}

	return s.signTypedData("AddPieces", message)
}

func (s *Signer) SignSchedulePieceRemovals(clientDataSetId *big.Int, pieceIds []*big.Int) (*eip712.AuthSignature, error) {
	message := map[string]interface{}{
		"clientDataSetId": clientDataSetId,
		"pieceIds":        pieceIds,
	}

	return s.signTypedData("SchedulePieceRemovals", message)
}

func (s *Signer) SignDeleteDataSet(clientDataSetId *big.Int) (*eip712.AuthSignature, error) {
	message := map[string]interface{}{
		"clientDataSetId": clientDataSetId,
	}

	return s.signTypedData("DeleteDataSet", message)
}

func (s *Signer) signTypedData(primaryType string, message map[string]interface{}) (*eip712.AuthSignature, error) {
	domain := eip712.GetDomain(s.chainId, s.verifyingContract)

	// Get the EIP-712 hash to sign
	hash, err := eip712.GetMessageHash(domain, primaryType, message)
	if err != nil {
		return nil, fmt.Errorf("failed to get message hash: %w", err)
	}

	// Sign the hash
	signature, err := crypto.Sign(hash, s.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign: %w", err)
	}

	// Verify we can recover the correct signer
	pubKey, err := crypto.SigToPub(hash, signature)
	if err != nil {
		return nil, fmt.Errorf("failed to recover public key: %w", err)
	}
	recoveredAddr := crypto.PubkeyToAddress(*pubKey)
	if recoveredAddr != s.address {
		return nil, fmt.Errorf("signature verification failed: expected %s, recovered %s", s.address.Hex(), recoveredAddr.Hex())
	}

	// Transform V from recovery ID to Ethereum signature standard
	// Ethereum uses 27/28, crypto.Sign returns 0/1
	v := signature[64] + 27

	// Extract r and s
	r := common.BytesToHash(signature[:32])
	sig_s := common.BytesToHash(signature[32:64])

	// Create full signature with adjusted V
	fullSig := make([]byte, 65)
	copy(fullSig[:32], signature[:32])
	copy(fullSig[32:64], signature[32:64])
	fullSig[64] = v

	return &eip712.AuthSignature{
		Signature:  fullSig,
		V:          v,
		R:          r,
		S:          sig_s,
		SignedData: hash,
		Signer:     s.address,
	}, nil
}
