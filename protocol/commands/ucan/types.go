package ucan

import "github.com/ipfs/go-cid"

type ConcludeArguments struct {
	Receipt cid.Cid `cborgen:"receipt" dagjsongen:"receipt"`
}

type RevokeArguments struct {
	// Revoke is the CID of the UCAN delegation to revoke.
	Revoke cid.Cid `cborgen:"revoke" dagjsongen:"revoke"`
	// Path is the delegation path to the UCAN delegation to revoke. The path is a
	// list of CIDs that represent the delegation chain from the root UCAN to the
	// UCAN being revoked.
	Path []cid.Cid `cborgen:"path" dagjsongen:"path"`
}
