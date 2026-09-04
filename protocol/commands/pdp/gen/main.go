//go:generate go run -tags codegen .

package main

import (
	"os"

	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/fil-forge/forge/protocol/commands/pdp"
)

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
		pdp.AcceptArguments{},
		pdp.AcceptOK{},
		pdp.InfoArguments{},
		pdp.InfoAcceptedAggregate{},
		pdp.InfoOK{},
	}
	const cborFile = "../cbor_gen.go"
	// merkletree.ProofData implements CBOR but not dag-json, and the stack is
	// CBOR-only on the wire, so we don't generate dag-json codecs here.
	if err := cbg.WriteMapEncodersToFile(cborFile, "pdp", models...); err != nil {
		panic(err)
	}
	tag(cborFile)
}
