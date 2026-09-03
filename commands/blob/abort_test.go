//go:build !codegen

package blob_test

import (
	"bytes"
	"testing"

	"github.com/fil-forge/forge/commands/blob"
	"github.com/fil-forge/libforge/testutil"
	"github.com/stretchr/testify/require"
)

// Round-trips AbortArguments through cbor.
func TestAbortArgumentsRoundTrip(t *testing.T) {
	in := blob.AbortArguments{
		Digest: testutil.RandomMultihash(t),
		Cause:  testutil.RandomCID(t),
	}
	var buf bytes.Buffer
	require.NoError(t, in.MarshalCBOR(&buf))
	var out blob.AbortArguments
	require.NoError(t, out.UnmarshalCBOR(&buf))
	require.Equal(t, in.Digest, out.Digest)
	require.Equal(t, in.Cause, out.Cause)
}

// Round-trips ReleaseArguments through cbor.
func TestReleaseArgumentsRoundTrip(t *testing.T) {
	in := blob.ReleaseArguments{
		Space:  testutil.RandomDID(t),
		Digest: testutil.RandomMultihash(t),
		Cause:  testutil.RandomCID(t),
	}
	var buf bytes.Buffer
	require.NoError(t, in.MarshalCBOR(&buf))
	var out blob.ReleaseArguments
	require.NoError(t, out.UnmarshalCBOR(&buf))
	require.Equal(t, in.Space, out.Space)
	require.Equal(t, in.Digest, out.Digest)
	require.Equal(t, in.Cause, out.Cause)
}

// Round-trips RejectArguments through cbor.
func TestRejectArgumentsRoundTrip(t *testing.T) {
	in := blob.RejectArguments{
		Space:  testutil.RandomDID(t),
		Digest: testutil.RandomMultihash(t),
	}
	var buf bytes.Buffer
	require.NoError(t, in.MarshalCBOR(&buf))
	var out blob.RejectArguments
	require.NoError(t, out.UnmarshalCBOR(&buf))
	require.Equal(t, in.Space, out.Space)
	require.Equal(t, in.Digest, out.Digest)
}
