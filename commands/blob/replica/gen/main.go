//go:generate go run -tags codegen .

package main

import (
	"os"

	"github.com/fil-forge/forge/commands/blob/replica"
	cbg "github.com/whyrusleeping/cbor-gen"
)

const buildTag = "//go:build !codegen\n\n"

func tag(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, append([]byte(buildTag), data...), 0o644); err != nil {
		panic(err)
	}
}

func main() {
	const cborFile = "../cbor_gen.go"
	if err := cbg.WriteMapEncodersToFile(cborFile, "replica",
		replica.AllocateArguments{},
		replica.AllocateOK{},
		replica.TransferArguments{},
		replica.TransferOK{},
	); err != nil {
		panic(err)
	}
	tag(cborFile)
}
