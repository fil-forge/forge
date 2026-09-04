//go:build !codegen

package upload

import (
	"github.com/fil-forge/forge/protocol/commands"
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

type RemoveOK = commands.Unit

// Remove (/upload/remove) deletes an upload's root→shards index entry on the
// upload service (subject = the space). It does NOT remove the shard blobs —
// blob removal is a separate per-digest /blob/remove decision owned by the
// client's reference accounting. Idempotent: removing an unknown root
// succeeds. The receipt carries no payload (Unit).
var Remove = binding.Bind[*RemoveArguments, *RemoveOK](command.MustParse("/upload/remove"))
