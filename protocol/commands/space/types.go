package space

import "github.com/fil-forge/ucantone/did"

type InfoOK struct {
	Providers []did.DID `cborgen:"providers" dagjsongen:"providers"`
}
