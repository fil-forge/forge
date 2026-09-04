package attestation

import (
	"fmt"

	"github.com/fil-forge/ucantone/varsig"
	"github.com/fil-forge/ucantone/varsig/algorithm"
	"github.com/multiformats/go-varint"
)

func init() {
	// Register spec-defined signature algorithms.
	varsig.RegisterAlgorithmScheme(algorithm.AlgorithmDef{
		Code:    VarsigCode,
		Name:    "Attested Authority",
		Decoder: DecodeAlgoithm,
	})
}

// VarsigCode is the Varsig signature algorithm code for attested signatures, under
// fil-one RFC 7. Note that the Varsig signature algorithm codes are *not*
// Multicodec codes! Officially, the Varsig code table makes no provision for
// extension, but we've selected a code in *Multicodec's* "private use" range,
// on the theory that it should be safe.
const VarsigCode uint64 = 0x300001

type Algorithm struct{}

var algorithmInstance algorithm.Algorithm = Algorithm{}

func (alg Algorithm) Encode() ([]byte, error) {
	return varint.ToUvarint(VarsigCode), nil
}

func DecodeAlgoithm(input []byte) (algorithm.Algorithm, int, error) {
	code, n, err := varint.FromUvarint(input)
	if err != nil {
		return nil, 0, err
	}
	if code != VarsigCode {
		return nil, n, fmt.Errorf("signature code is not attested-authority: 0x%02x, expected: 0x%02x", code, VarsigCode)
	}
	return algorithmInstance, n, nil
}
