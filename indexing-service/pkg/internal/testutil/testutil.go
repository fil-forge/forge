// Package testutil provides shared helpers for indexing service tests.
//
// It re-exports the ucantone testutil helpers (Alice, Bob, Service, RandomCID,
// RandomMultihash, RandomSigner, RandomDID, RandomBytes, Must, Must2, TestURL)
// and adds helpers for constructing self-signed UCAN invocations for the
// claims this service deals with (location, index, equals), random provider
// results, sharded DAG indexes, and structural equality assertions.
package testutil

import (
	"bytes"
	"math/rand"
	"net"
	"net/url"
	"strconv"
	"testing"

	"github.com/fil-forge/automobile"
	"github.com/fil-forge/forge/protocol/blobindex"
	"github.com/fil-forge/forge/protocol/commands"
	assertcaps "github.com/fil-forge/forge/protocol/commands/assert"
	"github.com/fil-forge/go-ipni-tools/pkg/metadata"
	"github.com/fil-forge/ucantone/testutil"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/ipfs/go-cid"
	"github.com/ipni/go-libipni/find/model"
	ipnimeta "github.com/ipni/go-libipni/metadata"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	mh "github.com/multiformats/go-multihash"
	multihash "github.com/multiformats/go-multihash/core"
	"github.com/stretchr/testify/require"
)

// Re-exports from ucantone/testutil for convenience and a single import point.
var (
	Alice           = testutil.Alice
	Bob             = testutil.Bob
	Carol           = testutil.Carol
	Mallory         = testutil.Mallory
	Service         = testutil.Service
	WebService      = testutil.WebService
	TestURL         = testutil.TestURL
	RandomBytes     = testutil.RandomBytes
	RandomCID       = testutil.RandomCID
	RandomMultihash = testutil.RandomDigest
	RandomSigner    = testutil.RandomSigner
	RandomIssuer    = testutil.RandomIssuer
)

func Must[T any](val T, err error) func(*testing.T) T { return testutil.Must(val, err) }
func Must2[T, U any](v1 T, v2 U, err error) func(*testing.T) (T, U) {
	return testutil.Must2(v1, v2, err)
}

func RandomMultihashes(t *testing.T, count int) []mh.Multihash {
	require.Greater(t, count, 0, "count must be greater than 0")
	mhs := make([]mh.Multihash, 0, count)
	for range count {
		mhs = append(mhs, RandomMultihash(t))
	}
	return mhs
}

func RandomPeer(t *testing.T) peer.ID {
	t.Helper()
	_, pub, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	pid, err := peer.IDFromPublicKey(pub)
	require.NoError(t, err)
	return pid
}

func RandomMultiaddr(t *testing.T) multiaddr.Multiaddr {
	t.Helper()
	addr := &net.TCPAddr{IP: net.IPv4(byte(rand.Intn(255)), byte(rand.Intn(255)), byte(rand.Intn(255)), byte(rand.Intn(255))), Port: rand.Intn(65535)}
	maddr, err := manet.FromIP(addr.IP)
	require.NoError(t, err)
	port, err := multiaddr.NewComponent(multiaddr.ProtocolWithCode(multiaddr.P_TCP).Name, strconv.Itoa(addr.Port))
	require.NoError(t, err)
	return multiaddr.Join(maddr, port)
}

// RandomCAR creates a single-block CAR of random bytes and returns the root
// CID, hash of the CAR bytes, and the CAR bytes.
func RandomCAR(t *testing.T, size int) (cid.Cid, mh.Multihash, []byte) {
	t.Helper()
	data := RandomBytes(t, size)
	root, err := cid.V1Builder{Codec: cid.Raw, MhType: multihash.SHA2_256}.Sum(data)
	require.NoError(t, err)
	var buf bytes.Buffer
	w := automobile.NewWriter(&buf)
	require.NoError(t, w.WriteHeader([]cid.Cid{root}))
	require.NoError(t, w.WriteBlock(automobile.Block{Link: root, Data: data}))
	carBytes := buf.Bytes()
	carDigest, err := mh.Sum(carBytes, mh.SHA2_256, -1)
	require.NoError(t, err)
	return root, carDigest, carBytes
}

// RandomLocationCommitment creates a self-signed /assert/location invocation
// (issuer == subject == a freshly-generated signer).
func RandomLocationCommitment(t *testing.T) ucan.Invocation {
	t.Helper()
	i := testutil.RandomIssuer(t)
	return Must(assertcaps.Location.Invoke(i, i.DID(), &assertcaps.LocationArguments{
		Space:    i.DID(),
		Content:  RandomMultihash(t),
		Location: []commands.CborURL{commands.CborURL(*TestURL)},
	}))(t)
}

// RandomIndexClaim creates a self-signed /assert/index invocation.
func RandomIndexClaim(t *testing.T) ucan.Invocation {
	t.Helper()
	i := testutil.RandomIssuer(t)
	inv, err := assertcaps.Index.Invoke(i, i.DID(), &assertcaps.IndexArguments{
		Index: RandomCID(t),
	})
	require.NoError(t, err)
	return inv
}

// RandomEqualsClaim creates a self-signed /assert/equals invocation.
func RandomEqualsClaim(t *testing.T) ucan.Invocation {
	t.Helper()
	i := testutil.RandomIssuer(t)
	inv, err := assertcaps.Equals.Invoke(i, i.DID(), &assertcaps.EqualsArguments{
		Content: RandomMultihash(t),
		Equals:  RandomCID(t),
	})
	require.NoError(t, err)
	return inv
}

// RandomShardedDagIndex creates a sharded DAG index with one shard containing
// a single slice of random bytes (size bytes). Returns the shard hash and the
// index.
func RandomShardedDagIndex(t *testing.T, size int) (mh.Multihash, blobindex.ShardedDagIndex) {
	t.Helper()
	shardHash := RandomMultihash(t)
	sliceHash := RandomMultihash(t)
	idx := blobindex.NewShardedDagIndex(1)
	idx.SetSlice(shardHash, sliceHash, blobindex.Range{Start: 0, End: int64(size - 1)})
	return shardHash, idx
}

// RandomProviderResult returns a randomly-populated IPNI provider result with
// no metadata.
func RandomProviderResult(t *testing.T) model.ProviderResult {
	t.Helper()
	return model.ProviderResult{
		ContextID: RandomBytes(t, 10),
		Metadata:  RandomBytes(t, 10),
		Provider: &peer.AddrInfo{
			ID:    RandomPeer(t),
			Addrs: []multiaddr.Multiaddr{RandomMultiaddr(t), RandomMultiaddr(t)},
		},
	}
}

// RandomBitswapProviderResult returns a provider result with bitswap metadata.
func RandomBitswapProviderResult(t *testing.T) model.ProviderResult {
	t.Helper()
	pr := RandomProviderResult(t)
	b, err := ipnimeta.Bitswap{}.MarshalBinary()
	require.NoError(t, err)
	pr.Metadata = b
	return pr
}

// RandomIndexClaimProviderResult returns a provider result with index-claim metadata.
func RandomIndexClaimProviderResult(t *testing.T) model.ProviderResult {
	t.Helper()
	meta := metadata.IndexClaimMetadata{
		Index:      RandomCID(t),
		Expiration: 0,
		Claim:      RandomCID(t),
	}
	b, err := meta.MarshalBinary()
	require.NoError(t, err)
	pr := RandomProviderResult(t)
	pr.Metadata = b
	return pr
}

// RandomLocationCommitmentProviderResult returns a provider result with
// location-commitment metadata.
func RandomLocationCommitmentProviderResult(t *testing.T) model.ProviderResult {
	t.Helper()
	shard := RandomCID(t)
	meta := metadata.LocationCommitmentMetadata{
		Shard:      &shard,
		Range:      &metadata.Range{Offset: 128},
		Expiration: 0,
		Claim:      RandomCID(t),
	}
	b, err := meta.MarshalBinary()
	require.NoError(t, err)
	pr := RandomProviderResult(t)
	pr.Metadata = b
	return pr
}

// RequireEqualClaim compares two claims by their canonical encoded
// bytes (and thus their CID). If both are nil, this passes.
func RequireEqualClaim(t *testing.T, expected, actual ucan.Invocation) {
	t.Helper()
	if expected == nil {
		require.Nil(t, actual)
		return
	}
	require.NotNil(t, actual)
	require.Equal(t, expected.Link(), actual.Link())
	require.Equal(t, expected.Bytes(), actual.Bytes(), "claim bytes differ")
}

// RequireEqualIndex compares two sharded DAG indexes structurally.
func RequireEqualIndex(t *testing.T, expected, actual blobindex.ShardedDagIndex) {
	t.Helper()
	if expected == nil {
		require.Nil(t, actual)
		return
	}
	require.NotNil(t, actual)
	require.NotZero(t, actual.Shards().Size())
	require.Equal(t, expected.Shards().Size(), actual.Shards().Size())
	for key, shard := range actual.Shards().Iterator() {
		require.True(t, expected.Shards().Has(key))
		exp := expected.Shards().Get(key)
		require.Equal(t, exp.Size(), shard.Size())
		for sliceMh, r := range shard.Iterator() {
			require.True(t, exp.Has(sliceMh))
			require.Equal(t, exp.Get(sliceMh), r)
		}
	}
}

// RandomLocalURL returns a random localhost URL useful as a service URL in
// tests where the actual port doesn't matter.
func RandomLocalURL(t *testing.T) url.URL {
	t.Helper()
	port := 3000 + rand.Intn(1000)
	u, err := url.Parse("http://127.0.0.1:" + strconv.Itoa(port))
	require.NoError(t, err)
	return *u
}
