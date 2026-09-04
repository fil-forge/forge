//go:generate go run -tags codegen .

package main

import (
	"os"

	jsg "github.com/alanshaw/dag-json-gen"
	"github.com/fil-forge/forge/protocol/commands/content"
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
	mapModels := []any{
		content.Blob{},
		content.RetrieveArguments{},
	}
	tupleModels := []any{
		content.Range{},
	}
	const (
		cborTuples = "../cbor_gen.tuples.go"
		cborMaps   = "../cbor_gen.maps.go"
		jsonTuples = "../json_gen.tuples.go"
		jsonMaps   = "../json_gen.maps.go"
	)
	if err := cbg.WriteTupleEncodersToFile(cborTuples, "content", tupleModels...); err != nil {
		panic(err)
	}
	if err := cbg.WriteMapEncodersToFile(cborMaps, "content", mapModels...); err != nil {
		panic(err)
	}
	if err := jsg.WriteTupleEncodersToFile(jsonTuples, "content", tupleModels...); err != nil {
		panic(err)
	}
	if err := jsg.WriteMapEncodersToFile(jsonMaps, "content", mapModels...); err != nil {
		panic(err)
	}
	tag(cborTuples)
	tag(cborMaps)
	tag(jsonTuples)
	tag(jsonMaps)
}
