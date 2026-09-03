package provider

import "github.com/fil-forge/ucantone/did"

type AddArguments struct {
	Provider did.DID `cborgen:"provider" dagjsongen:"provider"`
	Consumer did.DID `cborgen:"consumer" dagjsongen:"consumer"`
}

type AddOK struct {
	ID string `cborgen:"id" dagjsongen:"id"`
}
