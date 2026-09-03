// Package sign exposes the four /pdp/sign/* UCAN capabilities used by piri
// to drive an eip712 signing service:
//
//   - /pdp/sign/dataset/create
//   - /pdp/sign/dataset/delete
//   - /pdp/sign/pieces/add
//   - /pdp/sign/pieces/remove/schedule
//
// Each returns an [AuthSignature] (an eip712-signed bytes payload).
//
// Wire-level conventions:
//
//   - `*big.Int` fields (DataSet, Nonce, Pieces) are encoded as CBOR
//     bignums (tag 2). cbor-gen does not support negative bignums; the
//     domain (dataset IDs, nonces, piece IDs) is non-negative.
//
//   - `common.Address` (20 bytes) and `common.Hash` (32 bytes) are plain
//     `[]byte`; length validation is left to the caller.
package sign

import (
	"math/big"

	"github.com/ipfs/go-cid"
)

// AuthSignature is the eip712 auth signature returned from every
// /pdp/sign/* invocation. Field semantics mirror
// filecoin-services/go/eip712.AuthSignature.
type AuthSignature struct {
	Signature  []byte `cborgen:"signature" dagjsongen:"signature"`
	V          uint8  `cborgen:"v" dagjsongen:"v"`
	R          []byte `cborgen:"r" dagjsongen:"r"`
	S          []byte `cborgen:"s" dagjsongen:"s"`
	SignedData []byte `cborgen:"signedData" dagjsongen:"signedData"`
	Signer     []byte `cborgen:"signer" dagjsongen:"signer"`
}

// Metadata is the eip712 metadata bag attached to dataset & piece signing
// requests. Keys carries insertion order so the wire encoding is
// deterministic across implementations.
type Metadata struct {
	Keys   []string          `cborgen:"keys" dagjsongen:"keys"`
	Values map[string]string `cborgen:"values" dagjsongen:"values"`
}

// PieceProofs wraps the list of `blob/accept` invocation links proving the
// sub-pieces of one piece in a /pdp/sign/pieces/add request. cborgen does
// not natively encode `[][]cid.Cid`, so this single-level wrapper turns
// the outer slice into `[]PieceProofs`.
type PieceProofs struct {
	Proofs []cid.Cid `cborgen:"proofs" dagjsongen:"proofs"`
}

// DataSetCreateArguments are the arguments for /pdp/sign/dataset/create.
type DataSetCreateArguments struct {
	DataSet  *big.Int `cborgen:"dataSet" dagjsongen:"dataSet"`
	Payee    []byte   `cborgen:"payee" dagjsongen:"payee"`
	Metadata Metadata `cborgen:"metadata" dagjsongen:"metadata"`
}

// DataSetDeleteArguments are the arguments for /pdp/sign/dataset/delete.
type DataSetDeleteArguments struct {
	DataSet *big.Int `cborgen:"dataSet" dagjsongen:"dataSet"`
}

// PiecesAddArguments are the arguments for /pdp/sign/pieces/add.
//
// Each entry in PieceData has a matching Metadata entry and a matching
// Proofs entry. The Proofs[i].Proofs list links to `blob/accept`
// invocations for every sub-piece of piece i. Each `blob/accept` receipt
// MUST include the corresponding `pdp/accept` effect receipt; all receipt
// data MUST be attached to the signing invocation's container as proof
// blocks.
type PiecesAddArguments struct {
	DataSet   *big.Int      `cborgen:"dataSet" dagjsongen:"dataSet"`
	Nonce     *big.Int      `cborgen:"nonce" dagjsongen:"nonce"`
	PieceData [][]byte      `cborgen:"pieceData" dagjsongen:"pieceData"`
	Metadata  []Metadata    `cborgen:"metadata" dagjsongen:"metadata"`
	Proofs    []PieceProofs `cborgen:"proofs" dagjsongen:"proofs"`
}

// PiecesRemoveScheduleArguments are the arguments for
// /pdp/sign/pieces/remove/schedule.
type PiecesRemoveScheduleArguments struct {
	DataSet *big.Int   `cborgen:"dataSet" dagjsongen:"dataSet"`
	Pieces  []*big.Int `cborgen:"pieces" dagjsongen:"pieces"`
}
