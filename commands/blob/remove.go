//go:build !codegen

package blob

import (
	"github.com/fil-forge/forge/commands"
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

type RemoveOK = commands.Unit

// Remove (/blob/remove) releases a space's claim on an ACCEPTED blob.
//
// Served by the upload service (subject = the space). The service validates
// the caller's space authority, recovers every storage node holding the blob
// (the primary via the registration's receipt chain, plus any non-failed
// replicas), forwards a `/blob/release` to each, and deregisters the blob
// last — so the receipt chain to the primary survives for a retry if every
// forward fails. Forwarding is best-effort: the node-side handler is
// idempotent and unclaimed allocations expire, so a missed node is
// reconciled by provider-side hygiene.
//
// Parked (never-accepted) blobs are abandoned via `/blob/abort` instead.
//
// Idempotent: removing an unknown or already-removed blob succeeds. The
// receipt carries no payload (Unit).
var Remove = binding.Bind[*RemoveArguments, *RemoveOK](command.MustParse("/blob/remove"))
