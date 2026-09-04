package advertisement_test

import (
	"math/rand"
	"testing"

	"github.com/fil-forge/ucantone/testutil"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"
)

func randomMultihashes(t *testing.T, count int) []multihash.Multihash {
	require.Greater(t, count, 0, "count must be greater than 0")
	mhs := make([]multihash.Multihash, 0, count)
	for range count {
		mhs = append(mhs, testutil.RandomDigest(t))
	}
	return mhs
}

var seedSeq int64

// randomPeer derives a peer ID from a deterministic key sequence, so test
// output is stable across runs.
func randomPeer(t *testing.T) peer.ID {
	src := rand.NewSource(seedSeq)
	seedSeq++
	_, publicKey := testutil.Must2(crypto.GenerateEd25519Key(rand.New(src)))(t)
	return testutil.Must(peer.IDFromPublicKey(publicKey))(t)
}
