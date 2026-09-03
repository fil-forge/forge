// Package commands defines UCAN commands used across the forge
// stack. Each operation lives in a subpackage (e.g. blob, upload, pdp);
// this top-level package holds shared wire types and helpers.
package commands

// Unit is the empty wire type returned by any capability whose receipt
// carries no payload (e.g. /upload/remove, /claim/cache). It encodes as
// an empty CBOR map / dag-json object.
type Unit struct{}
