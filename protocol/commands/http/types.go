package http

import (
	"github.com/fil-forge/forge/protocol/commands/blob"
	"github.com/fil-forge/ucantone/ucan/promise"
)

type PutArguments struct {
	Body blob.Blob `cborgen:"body" dagjsongen:"body"`
	// Destination is the promise that resolves to the upload destination
	// where the blob should be PUT to. It is the result of a /blob/allocate task.
	Destination promise.AwaitOK `cborgen:"destination" dagjsongen:"destination"`
}
