package s3_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/fil-forge/forge/protocol/commands/s3"
	"github.com/fil-forge/forge/protocol/commands/s3/bucket"
	"github.com/fil-forge/forge/protocol/commands/s3/request"
	"github.com/fil-forge/ucantone/did"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
)

func mustCid(t *testing.T, s string) cid.Cid {
	t.Helper()
	c, err := cid.Parse(s)
	if err != nil {
		t.Fatalf("parsing cid %q: %v", s, err)
	}
	return c
}

func ptr[T any](v T) *T {
	return &v
}

func TestRequestRoundTrip(t *testing.T) {
	in := &s3.Request{
		Method: "GET",
		Headers: map[string]string{
			"host":         "region.s3.fil.one",
			"x-amz-header": "a",
		},
		URL: "https://region.s3.fil.one/bucket/path?x-id=GetObject",
	}

	// CBOR
	var cb bytes.Buffer
	if err := in.MarshalCBOR(&cb); err != nil {
		t.Fatalf("MarshalCBOR: %v", err)
	}
	var outCBOR s3.Request
	if err := outCBOR.UnmarshalCBOR(bytes.NewReader(cb.Bytes())); err != nil {
		t.Fatalf("UnmarshalCBOR: %v", err)
	}
	if !reflect.DeepEqual(*in, outCBOR) {
		t.Fatalf("CBOR round-trip mismatch:\n got %#v\nwant %#v", outCBOR, *in)
	}

	// DAG-JSON
	var jb bytes.Buffer
	if err := in.MarshalDagJSON(&jb); err != nil {
		t.Fatalf("MarshalDagJSON: %v", err)
	}
	var outJSON s3.Request
	if err := outJSON.UnmarshalDagJSON(bytes.NewReader(jb.Bytes())); err != nil {
		t.Fatalf("UnmarshalDagJSON: %v\njson: %s", err, jb.String())
	}
	if !reflect.DeepEqual(*in, outJSON) {
		t.Fatalf("DAG-JSON round-trip mismatch:\n got %#v\nwant %#v", outJSON, *in)
	}
	// DAG-JSON keys are emitted in lexicographic order.
	if got := jb.String(); !strings.HasPrefix(got, `{"headers":`) {
		t.Fatalf("expected headers first in DAG-JSON, got: %s", got)
	}
}

func TestAuthorizeOKRoundTrip(t *testing.T) {
	access := did.MustParse("did:key:z6MkjFRxLLGdBqQSLkZbVnuwUFiomK8eGBkPtim9ETvP7vec")
	delCid := mustCid(t, "bafyreienos3cw7hcga5vwani3pberioe2qscnz5jk2um5jajo4v7bwmvvm")

	in := &request.AuthorizeOK{
		Bucket: ptr(did.MustParse("did:key:z6MkmNBgCewjYfEDTdKLpHkbMWUogJk29CxmiVdLeW4Kz3UG")),
		Tenant: did.MustParse("did:plc:ewvi7nxzyoun6zhxrhs64oiz"),
		Permissions: s3.PermissionSet{Entries: map[did.DID][]string{
			access: {"s3:GetObject", "s3:PutObject"},
		}},
		Keys: s3.KeySet{Entries: map[did.DID][]s3.VerificationKey{
			access: {{Kind: s3.KeyKindSigV4a, Data: []byte{1, 2, 3, 4}}},
		}},
		Delegations: s3.ProofSet{Entries: map[cid.Cid][]cid.Cid{
			delCid: {delCid},
		}},
	}

	var cb bytes.Buffer
	if err := in.MarshalCBOR(&cb); err != nil {
		t.Fatalf("MarshalCBOR: %v", err)
	}
	var outCBOR request.AuthorizeOK
	if err := outCBOR.UnmarshalCBOR(bytes.NewReader(cb.Bytes())); err != nil {
		t.Fatalf("UnmarshalCBOR: %v", err)
	}
	if !reflect.DeepEqual(*in, outCBOR) {
		t.Fatalf("CBOR round-trip mismatch:\n got %#v\nwant %#v", outCBOR, *in)
	}

	var jb bytes.Buffer
	if err := in.MarshalDagJSON(&jb); err != nil {
		t.Fatalf("MarshalDagJSON: %v", err)
	}
	var outJSON request.AuthorizeOK
	if err := outJSON.UnmarshalDagJSON(bytes.NewReader(jb.Bytes())); err != nil {
		t.Fatalf("UnmarshalDagJSON: %v\njson: %s", err, jb.String())
	}
	if !reflect.DeepEqual(*in, outJSON) {
		t.Fatalf("DAG-JSON round-trip mismatch:\n got %#v\nwant %#v", outJSON, *in)
	}
}

func TestAuthorizeOKEmptyValuesRoundTrip(t *testing.T) {
	// Nil/empty slice-valued maps must encode as empty objects and decode back.
	in := &request.AuthorizeOK{
		Permissions: s3.PermissionSet{},
		Keys:        s3.KeySet{},
		Delegations: s3.ProofSet{},
	}
	var jb bytes.Buffer
	if err := in.MarshalDagJSON(&jb); err != nil {
		t.Fatalf("MarshalDagJSON: %v", err)
	}
	var out request.AuthorizeOK
	if err := out.UnmarshalDagJSON(bytes.NewReader(jb.Bytes())); err != nil {
		t.Fatalf("UnmarshalDagJSON: %v\njson: %s", err, jb.String())
	}
	require.Nil(t, out.Bucket)
	if len(out.Permissions.Entries) != 0 || len(out.Keys.Entries) != 0 || len(out.Delegations.Entries) != 0 {
		t.Fatalf("expected empty maps, got %#v", out)
	}
}

func TestInfoOKRoundTrip(t *testing.T) {
	access := did.MustParse("did:key:z6MkjFRxLLGdBqQSLkZbVnuwUFiomK8eGBkPtim9ETvP7vec")
	root := mustCid(t, "bafyreiabuvg5hkupzjfn2kqywbdp5xhsb25pmhviyfz77yxhspssvxsv5y")
	inter := mustCid(t, "bafyreigngbemvzgbmelwddwoms2ak2g32vmhcpxg6dqlwvb6spiezoc4py")

	in := &bucket.InfoOK{
		ID: did.MustParse("did:key:z6MkmNBgCewjYfEDTdKLpHkbMWUogJk29CxmiVdLeW4Kz3UG"),
		Permissions: s3.PermissionSet{Entries: map[did.DID][]string{
			access: {"s3:GetObject", "s3:PutObject"},
		}},
		Delegations: s3.ProofSet{Entries: map[cid.Cid][]cid.Cid{
			inter: {root, inter},
		}},
	}

	var cb bytes.Buffer
	if err := in.MarshalCBOR(&cb); err != nil {
		t.Fatalf("MarshalCBOR: %v", err)
	}
	var outCBOR bucket.InfoOK
	if err := outCBOR.UnmarshalCBOR(bytes.NewReader(cb.Bytes())); err != nil {
		t.Fatalf("UnmarshalCBOR: %v", err)
	}
	if !reflect.DeepEqual(*in, outCBOR) {
		t.Fatalf("CBOR round-trip mismatch:\n got %#v\nwant %#v", outCBOR, *in)
	}

	var jb bytes.Buffer
	if err := in.MarshalDagJSON(&jb); err != nil {
		t.Fatalf("MarshalDagJSON: %v", err)
	}
	var outJSON bucket.InfoOK
	if err := outJSON.UnmarshalDagJSON(bytes.NewReader(jb.Bytes())); err != nil {
		t.Fatalf("UnmarshalDagJSON: %v\njson: %s", err, jb.String())
	}
	if !reflect.DeepEqual(*in, outJSON) {
		t.Fatalf("DAG-JSON round-trip mismatch:\n got %#v\nwant %#v", outJSON, *in)
	}
}
