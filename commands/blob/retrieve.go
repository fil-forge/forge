//go:build !codegen

package blob

import (
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

// Retrieve is the service-level retrieval capability (e.g. used by the
// indexer to fetch content claims from a Piri node). It is NOT space-scoped:
// any holder of a valid delegation for `/blob/retrieve` may fetch the blob
// by digest, regardless of which space it was originally stored under.
//
// For user-facing retrieval that requires an allocation in a specific space
// see `libforge/commands/content.Retrieve` (the `/content/retrieve`
// capability).
var Retrieve = binding.Bind[*RetrieveArguments, *RetrieveOK](command.MustParse("/blob/retrieve"))
