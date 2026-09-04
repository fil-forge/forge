package blobindex

import (
	"bytes"
	"fmt"
	"io"
	"slices"

	"github.com/fil-forge/automobile"
	dm "github.com/fil-forge/forge/protocol/blobindex/datamodel"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

// Extract extracts a sharded dag index from a car
func Extract(r io.Reader) (*MapShardedDagIndex, error) {
	dc, err := decodeIndexCar(r)
	if err != nil {
		return nil, NewDecodeFailureError(err)
	}
	return View(dc.root, dc.blocks)
}

// indexCar is the decoded view of an index CAR file.
type indexCar struct {
	root   cid.Cid
	blocks map[cid.Cid][]byte
}

func decodeIndexCar(r io.Reader) (indexCar, error) {
	roots, blocks, err := automobile.Decode(r)
	if err != nil {
		return indexCar{}, fmt.Errorf("decoding index CAR: %w", err)
	}
	if len(roots) != 1 {
		return indexCar{}, fmt.Errorf("expected exactly one root, got: %d", len(roots))
	}
	codec := roots[0].Prefix().Codec
	if codec != cid.DagCBOR {
		return indexCar{}, fmt.Errorf("unexpected root CID codec: %x", codec)
	}
	data := indexCar{root: roots[0], blocks: map[cid.Cid][]byte{}}
	for _, blk := range blocks {
		data.blocks[blk.Link] = blk.Data
	}
	if _, ok := data.blocks[data.root]; !ok {
		return indexCar{}, fmt.Errorf("missing root block: %s", data.root)
	}
	return data, nil
}

func View(root cid.Cid, blocks map[cid.Cid][]byte) (*MapShardedDagIndex, error) {
	rootBlock, ok := blocks[root]
	if !ok {
		return nil, NewDecodeFailureError(fmt.Errorf("missing root block: %s", root))
	}

	var shardedDagIndexData dm.ShardedDagIndexModel
	if err := shardedDagIndexData.UnmarshalCBOR(bytes.NewReader(rootBlock)); err != nil {
		return nil, NewDecodeFailureError(fmt.Errorf("decoding root block: %s: %v", root, err))
	}
	if shardedDagIndexData.DagO_1 == nil {
		return nil, NewUnknownFormatError(fmt.Errorf("unknown index version"))
	}

	dagIndex := NewShardedDagIndex(len(shardedDagIndexData.DagO_1.Shards))
	for _, shardLink := range shardedDagIndexData.DagO_1.Shards {
		shard, ok := blocks[shardLink]
		if !ok {
			return nil, NewDecodeFailureError(fmt.Errorf("missing shard block: %s", shardLink))
		}
		var blobIndexData dm.BlobIndexModel
		err := blobIndexData.UnmarshalCBOR(bytes.NewReader(shard))
		if err != nil {
			return nil, NewDecodeFailureError(err)
		}
		blobIndex := NewMultihashMap[Range](len(blobIndexData.Slices))
		for _, blobSlice := range blobIndexData.Slices {
			blobIndex.Set(blobSlice.Digest, blobSlice.Range)
		}
		dagIndex.Shards().Set(blobIndexData.Digest, blobIndex)
	}
	return dagIndex, nil
}

// MapShardedDagIndex is an in-memory implementation of ShardedDagIndex
// using [MultihashMap].
type MapShardedDagIndex struct {
	shards MultihashMap[MultihashMap[Range]]
}

// NewShardedDagIndex constructs an empty sharded DAG index.
// shardSizeHint is used to preallocate the number of shards that will be added.
// Set to -1 if unknown.
func NewShardedDagIndex(shardSizeHint int) *MapShardedDagIndex {
	return &MapShardedDagIndex{NewMultihashMap[MultihashMap[Range]](shardSizeHint)}
}

func (sdi *MapShardedDagIndex) Shards() MultihashMap[MultihashMap[Range]] {
	return sdi.shards
}

func (sdi *MapShardedDagIndex) SetSlice(shard mh.Multihash, slice mh.Multihash, byteRange Range) {
	index := sdi.shards.Get(shard)
	if index == nil {
		index = NewMultihashMap[Range](-1)
		sdi.shards.Set(shard, index)
	}
	index.Set(slice, byteRange)
}

func (sdi *MapShardedDagIndex) Archive(w io.Writer) error {
	return Archive(sdi, w)
}

// NewUnknownFormatError returns an error for an unknown format.
func NewUnknownFormatError(reason error) error {
	return fmt.Errorf("unknown format: %w", reason)
}

// NewDecodeFailureError returns an error for a decode failure.
func NewDecodeFailureError(reason error) error {
	return fmt.Errorf("decode failure: %w", reason)
}

// Archive writes a ShardedDagIndex to a CAR file
func Archive(index ShardedDagIndex, writer io.Writer) error {
	// assemble blob index shards
	blobIndexDatas, err := toList(index.Shards(), func(shardHash mh.Multihash, shard MultihashMap[Range]) (dm.BlobIndexModel, error) {
		// assemble blob slices
		blobSliceDatas, err := toList(shard, func(sliceHash mh.Multihash, byteRange Range) (dm.BlobSliceModel, error) {
			return dm.BlobSliceModel{Digest: sliceHash, Range: byteRange}, nil
		})
		if err != nil {
			return dm.BlobIndexModel{}, err
		}
		// sort blob slices
		if err := sortByDigest(blobSliceDatas, func(bsm dm.BlobSliceModel) mh.Multihash {
			return bsm.Digest
		}); err != nil {
			return dm.BlobIndexModel{}, err
		}
		return dm.BlobIndexModel{
			Digest: shardHash,
			Slices: blobSliceDatas,
		}, nil
	})
	if err != nil {
		return err
	}
	// sort blob index shards
	if err := sortByDigest(blobIndexDatas, func(bim dm.BlobIndexModel) mh.Multihash {
		return bim.Digest
	}); err != nil {
		return err
	}

	// initialize root sharded dag index
	shardedDagIndex := dm.ShardedDagIndexModel_0_1{
		Shards: make([]cid.Cid, 0, len(blobIndexDatas)),
	}
	// encode blob index shards to blocks and add links to sharded dag index
	blks := make([]automobile.Block, 0, len(blobIndexDatas)+1)
	for _, shard := range blobIndexDatas {
		var buf bytes.Buffer
		err := shard.MarshalCBOR(&buf)
		if err != nil {
			return err
		}
		b := buf.Bytes()
		l, err := cid.V1Builder{Codec: cid.DagCBOR, MhType: mh.SHA2_256}.Sum(b)
		if err != nil {
			return err
		}
		blks = append(blks, automobile.Block{Data: b, Link: l})
		shardedDagIndex.Shards = append(shardedDagIndex.Shards, l)
	}

	// encode the root block
	model := dm.ShardedDagIndexModel{DagO_1: &shardedDagIndex}
	var rootData bytes.Buffer
	if err := model.MarshalCBOR(&rootData); err != nil {
		return err
	}
	root, err := cid.V1Builder{Codec: cid.DagCBOR, MhType: mh.SHA2_256}.Sum(rootData.Bytes())
	if err != nil {
		return err
	}
	rootBlock := automobile.Block{Link: root, Data: rootData.Bytes()}

	carWriter := automobile.NewWriter(writer)
	if err := carWriter.WriteHeader([]cid.Cid{root}); err != nil {
		return fmt.Errorf("writing CAR header: %w", err)
	}
	for _, blk := range append(blks, rootBlock) {
		if err := carWriter.WriteBlock(blk); err != nil {
			return fmt.Errorf("writing CAR block: %w", err)
		}
	}
	return nil
}

func toList[E, T any](mhm MultihashMap[T], newElem func(mh.Multihash, T) (E, error)) ([]E, error) {
	asList := make([]E, 0, mhm.Size())
	for hash, value := range mhm.Iterator() {
		e, err := newElem(hash, value)
		if err != nil {
			return nil, err
		}
		asList = append(asList, e)
	}
	return asList, nil
}

func sortByDigest[E any](list []E, getDigest func(E) mh.Multihash) error {
	decodeds := NewMultihashMap[*mh.DecodedMultihash](len(list))
	for _, e := range list {
		hash := getDigest(e)
		decoded, err := mh.Decode(hash)
		if err != nil {
			return err
		}
		decodeds.Set(hash, decoded)
	}
	slices.SortFunc(list, func(a, b E) int {
		decodedA := decodeds.Get(getDigest(a))
		decodedB := decodeds.Get(getDigest(b))
		return bytes.Compare(decodedA.Digest, decodedB.Digest)
	})
	return nil
}
