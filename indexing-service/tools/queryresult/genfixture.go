// Generates an archive of a query result that contains 1 index and 3 claims.
//
// The claims consist of 2 location claims, (for the index and a CAR blob) and
// 1 index claim, asserting the CID of the CAR blob is indexed by the index.
//
// The filename is for the content root multihash (the root of the DAG in the
// CAR blob).
package main

import (
	"bytes"
	crand "crypto/rand"
	"fmt"
	"io"
	"math/rand/v2"
	"net/url"
	"os"

	"github.com/fil-forge/automobile"
	"github.com/fil-forge/forge/indexing-service/pkg/service/queryresult"
	"github.com/fil-forge/forge/indexing-service/pkg/types"
	"github.com/fil-forge/forge/protocol/blobindex"
	"github.com/fil-forge/forge/protocol/bytemap"
	ctypes "github.com/fil-forge/forge/protocol/commands"
	cassert "github.com/fil-forge/forge/protocol/commands/assert"
	"github.com/fil-forge/ucantone/multikey"
	"github.com/fil-forge/ucantone/multikey/ed25519"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/fil-forge/ucantone/ucan/invocation"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multibase"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
)

func main() {
	storageID := multikey.KeyIssuer(must(ed25519.Generate()))
	uploadServiceID := multikey.KeyIssuer(must(ed25519.Generate()))
	indexingServiceID := multikey.KeyIssuer(must(ed25519.Generate()))
	space := multikey.KeyIssuer(must(ed25519.Generate()))
	spaceDID := space.DID()

	blockBytes := randomBytes(32)
	blockDigest := must(multihash.Sum(blockBytes, multihash.SHA2_256, -1))
	blockLink := cid.NewCidV1(cid.Raw, blockDigest)

	carRoots := []cid.Cid{blockLink}
	carBlock := automobile.Block{Link: blockLink, Data: blockBytes}
	carBytes := must(io.ReadAll(automobile.Encode(carRoots, []automobile.Block{carBlock})))
	carDigest := must(multihash.Sum(carBytes, multihash.SHA2_256, -1))

	_, ents, err := automobile.Index(bytes.NewReader(carBytes))
	if err != nil {
		panic(err)
	}
	index := blobindex.NewShardedDagIndex(1)
	for _, ent := range ents {
		index.SetSlice(carDigest, ent.Link.Hash(), blobindex.Range{Start: int64(ent.Start), End: int64(ent.End)})
	}
	var indexBuf bytes.Buffer
	if err := blobindex.Archive(index, &indexBuf); err != nil {
		panic(err)
	}
	indexBytes := indexBuf.Bytes()
	indexDigest := must(multihash.Sum(indexBytes, multihash.SHA2_256, -1))
	indexLink := cid.NewCidV1(uint64(multicodec.Car), indexDigest)

	carLocationURL := blobURL(randomURL(), carDigest)
	carLocationCommitment := must(cassert.Location.Invoke(
		storageID,
		storageID.DID(),
		&cassert.LocationArguments{
			Space:    space.DID(),
			Content:  carDigest,
			Location: []ctypes.CborURL{carLocationURL},
		},
		invocation.WithAudience(space.DID()),
	))

	indexLocationURL := blobURL(randomURL(), indexDigest)
	indexLocationCommitment := must(cassert.Location.Invoke(
		storageID,
		storageID.DID(),
		&cassert.LocationArguments{
			Space:    space.DID(),
			Content:  indexDigest,
			Location: []ctypes.CborURL{indexLocationURL},
		},
		invocation.WithAudience(space.DID()),
	))

	indexClaim := must(cassert.Index.Invoke(
		uploadServiceID,
		uploadServiceID.DID(),
		&cassert.IndexArguments{
			Index: indexLink,
		},
		invocation.WithAudience(indexingServiceID.DID()),
	))

	claims := map[cid.Cid]ucan.Invocation{
		carLocationCommitment.Link():   carLocationCommitment,
		indexLocationCommitment.Link(): indexLocationCommitment,
		indexClaim.Link():              indexClaim,
	}

	indexes := bytemap.NewByteMap[types.EncodedContextID, blobindex.ShardedDagIndex](1)
	indexes.Set(
		must(types.ContextID{Space: &spaceDID, Hash: indexDigest}.ToEncoded()),
		index,
	)

	result := must(queryresult.Build(claims, indexes))

	var resultBuf bytes.Buffer
	writer := automobile.NewWriter(&resultBuf)
	writer.WriteHeader([]cid.Cid{result.Root().Link})
	for _, blk := range result.Blocks() {
		if err := writer.WriteBlock(automobile.Block(blk)); err != nil {
			panic(err)
		}
	}

	// verify result can be read
	must(queryresult.Extract(&resultBuf))

	filename := fmt.Sprintf("%s.queryresult.car", must(multibase.Encode(multibase.Base58BTC, blockDigest)))
	if err := os.WriteFile(filename, resultBuf.Bytes(), 0644); err != nil {
		panic(err)
	}
}

func must[T any](val T, err error) T {
	if err != nil {
		panic(err)
	}
	return val
}

func randomBytes(size int) []byte {
	bytes := make([]byte, size)
	must(crand.Read(bytes))
	return bytes
}

func randomURL() *url.URL {
	port := 3000 + rand.IntN(1000)
	return must(url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port)))
}

func blobURL(base *url.URL, digest multihash.Multihash) ctypes.CborURL {
	url := base.JoinPath("/blob/%s", must(multibase.Encode(multibase.Base58BTC, digest)))
	return ctypes.CborURL(*url)
}
