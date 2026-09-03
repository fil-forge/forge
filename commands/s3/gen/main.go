//go:generate go run -tags codegen .

package main

import (
	"os"

	jsg "github.com/alanshaw/dag-json-gen"
	"github.com/fil-forge/forge/commands/s3"
	cbg "github.com/whyrusleeping/cbor-gen"
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
	// PermissionSet, KeySet and ProofSet have hand-written codecs (see
	// codec.go) because cbor-gen / dag-json-gen do not support slice-valued
	// maps.
	models := []any{
		s3.Request{},
		s3.VerificationKey{},
	}
	const (
		cborFile = "../cbor_gen.go"
		jsonFile = "../json_gen.go"
	)
	if err := cbg.WriteMapEncodersToFile(cborFile, "s3", models...); err != nil {
		panic(err)
	}
	if err := jsg.WriteMapEncodersToFile(jsonFile, "s3", models...); err != nil {
		panic(err)
	}
	tag(cborFile)
	tag(jsonFile)
}
