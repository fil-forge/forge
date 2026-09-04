//go:build !codegen

package blob

import (
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

// MaxBlobSize is the network-wide ceiling on a single blob, in bytes, as
// shipped to a storage node: the most raw bytes a default-configured piri
// accepts in one piece. Piri's default piece ceiling is a 256 MiB PADDED
// piece (piri pkg/pdp/piecesize, DefaultMaxPaddedSize), and Fr32 padding
// leaves 127/128 of it for data — 266338304 (~254 MiB). Piri's piece config
// is raise-only (an operator may accept more, never less), so a client
// sharding uploads at this constant is safe against every conforming node.
//
// This measures the stored artifact. An encrypting client must choose its
// plaintext split below this, leaving room for its envelope framing.
const MaxBlobSize = (256 << 20) * 127 / 128

var Allocate = binding.Bind[*AllocateArguments, *AllocateOK](command.MustParse("/blob/allocate"))
