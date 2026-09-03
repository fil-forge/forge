//go:generate go run -tags codegen .

package main

import (
	"os"

	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/fil-forge/forge/commands/pdp/sign"
)

// The parent sign package has bindcap.New calls that require codec methods
// to exist on the argument types. Those codecs are what this tool
// generates. To break the bootstrap, bindings and generated codec files
// carry `//go:build !codegen`; this tool is built with `-tags codegen`, so
// the import of `sign` here only pulls in the wire types from types.go.
//
// After cbor-gen writes the codec file, we re-tag it with the same
// constraint so subsequent codegen runs are stale-safe.
const buildTag = "//go:build !codegen\n\n"

func tag(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, append([]byte(buildTag), data...), 0644); err != nil {
		panic(err)
	}
}

func main() {
	models := []any{
		sign.AuthSignature{},
		sign.Metadata{},
		sign.PieceProofs{},
		sign.DataSetCreateArguments{},
		sign.DataSetDeleteArguments{},
		sign.PiecesAddArguments{},
		sign.PiecesRemoveScheduleArguments{},
	}
	const cborFile = "../cbor_gen.go"
	// The stack is CBOR-only on the wire, so we don't generate dag-json codecs.
	if err := cbg.WriteMapEncodersToFile(cborFile, "sign", models...); err != nil {
		panic(err)
	}
	tag(cborFile)
}
