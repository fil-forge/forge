//go:generate go run -tags codegen .

package main

import (
	"os"

	jsg "github.com/alanshaw/dag-json-gen"
	"github.com/fil-forge/forge/commands/blob"
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
	models := []any{
		blob.AllocateArguments{},
		blob.Blob{},
		blob.AllocateOK{},
		blob.BlobAddress{},
		blob.AcceptArguments{},
		blob.AcceptOK{},
		blob.AddArguments{},
		blob.AddOK{},
		blob.RemoveArguments{},
		blob.ReleaseArguments{},
		blob.AbortArguments{},
		blob.RejectArguments{},
		blob.ReplicateArguments{},
		blob.ReplicateOK{},
		blob.ListArguments{},
		blob.ListOK{},
		blob.ListBlobItem{},
		blob.RetrieveArguments{},
		blob.RetrieveBlob{},
		blob.RetrieveOK{},
	}
	const (
		cborFile = "../cbor_gen.go"
		jsonFile = "../json_gen.go"
	)
	if err := cbg.WriteMapEncodersToFile(cborFile, "blob", models...); err != nil {
		panic(err)
	}
	if err := jsg.WriteMapEncodersToFile(jsonFile, "blob", models...); err != nil {
		panic(err)
	}
	tag(cborFile)
	tag(jsonFile)
}
