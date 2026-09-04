package client

import (
	"math/big"
	"net/http"
	"net/url"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/fil-forge/filecoin-services/go/eip712"
	"github.com/fil-forge/forge/protocol/commands/pdp/sign"
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/client"
	uerrors "github.com/fil-forge/ucantone/errors"
	"github.com/fil-forge/ucantone/server"
	"github.com/fil-forge/ucantone/testutil"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mustDelegate issues a delegation from `issuer` to `audience.DID()` for
// `cap`, with the issuer's own DID as the subject. Bails the test if
// delegation fails.
func mustDelegate[Args, OK binding.CBORValue](
	t *testing.T,
	cap binding.Binding[Args, OK],
	issuer ucan.Issuer,
	audience ucan.Principal,
) ucan.Delegation {
	t.Helper()
	dlg, err := cap.Delegate(issuer, audience.DID(), issuer.DID())
	require.NoError(t, err)
	return dlg
}

// newClientBackedByServer wires a ucantone HTTPClient to use the supplied
// HTTPServer as its transport. That way invocations go through the full
// encode → decode → handle → encode response → decode cycle without
// listening on a real port.
func newClientBackedByServer(t *testing.T, srv *server.HTTPServer, service ucan.Issuer) *Client {
	t.Helper()
	endpoint, err := url.Parse("http://test")
	require.NoError(t, err)
	httpClient, err := client.NewHTTP(endpoint, client.WithHTTPClient(&http.Client{Transport: srv}))
	require.NoError(t, err)
	return &Client{ServiceDID: service.DID(), HTTP: httpClient}
}

func TestClient_SignCreateDataSet(t *testing.T) {
	mock := mockLibforgeSignature()
	service := testutil.RandomIssuer(t)
	alice := testutil.RandomIssuer(t)

	srv := server.NewHTTP(service)
	srv.Handle(sign.DataSetCreate.Command, binding.NewHandler(
		func(req *binding.Request[*sign.DataSetCreateArguments], res *binding.Response[*sign.DataSetCreateOK]) error {
			args := req.Task().Arguments()
			assert.Equal(t, "12345", args.DataSet.String())
			assert.Equal(t, "0xabCDEF1234567890ABcDEF1234567890aBCDeF12", common.BytesToAddress(args.Payee).String())
			assert.Len(t, args.Metadata.Keys, 1)
			assert.Equal(t, "test-key", args.Metadata.Keys[0])
			return res.SetSuccess(&mock)
		},
	))

	c := newClientBackedByServer(t, srv, service)
	dlg := mustDelegate(t, sign.DataSetCreate, service, alice)

	signature, err := c.SignCreateDataSet(
		t.Context(),
		alice,
		big.NewInt(12345),
		common.HexToAddress("0xabcdef1234567890abcdef1234567890abcdef12"),
		[]eip712.MetadataEntry{{Key: "test-key", Value: "test-value"}},
		[]ucan.Delegation{dlg},
	)
	require.NoError(t, err)
	require.NotNil(t, signature)
	assert.Equal(t, common.BytesToAddress(mock.Signer), signature.Signer)
}

func TestClient_SignAddPieces(t *testing.T) {
	mock := mockLibforgeSignature()
	service := testutil.RandomIssuer(t)
	alice := testutil.RandomIssuer(t)

	srv := server.NewHTTP(service)
	srv.Handle(sign.PiecesAdd.Command, binding.NewHandler(
		func(req *binding.Request[*sign.PiecesAddArguments], res *binding.Response[*sign.PiecesAddOK]) error {
			args := req.Task().Arguments()
			assert.Equal(t, "12345", args.DataSet.String())
			assert.Equal(t, "0", args.Nonce.String())
			assert.Len(t, args.PieceData, 2)
			assert.Equal(t, []byte("piece1"), args.PieceData[0])
			return res.SetSuccess(&mock)
		},
	))

	c := newClientBackedByServer(t, srv, service)
	dlg := mustDelegate(t, sign.PiecesAdd, service, alice)

	signature, err := c.SignAddPieces(
		t.Context(),
		alice,
		big.NewInt(12345),
		big.NewInt(0),
		[][]byte{[]byte("piece1"), []byte("piece2")},
		[][]eip712.MetadataEntry{
			{{Key: "piece1-key", Value: "piece1-value"}},
			{{Key: "piece2-key", Value: "piece2-value"}},
		},
		nil, // pieceProofs
		nil, // proofContainer
		[]ucan.Delegation{dlg},
	)
	require.NoError(t, err)
	require.NotNil(t, signature)
	assert.Equal(t, common.BytesToAddress(mock.Signer), signature.Signer)
}

func TestClient_SignSchedulePieceRemovals(t *testing.T) {
	mock := mockLibforgeSignature()
	service := testutil.RandomIssuer(t)
	alice := testutil.RandomIssuer(t)

	srv := server.NewHTTP(service)
	srv.Handle(sign.PiecesRemoveSchedule.Command, binding.NewHandler(
		func(req *binding.Request[*sign.PiecesRemoveScheduleArguments], res *binding.Response[*sign.PiecesRemoveScheduleOK]) error {
			args := req.Task().Arguments()
			assert.Equal(t, "12345", args.DataSet.String())
			assert.Len(t, args.Pieces, 3)
			return res.SetSuccess(&mock)
		},
	))

	c := newClientBackedByServer(t, srv, service)
	dlg := mustDelegate(t, sign.PiecesRemoveSchedule, service, alice)

	signature, err := c.SignSchedulePieceRemovals(
		t.Context(),
		alice,
		big.NewInt(12345),
		[]*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)},
		[]ucan.Delegation{dlg},
	)
	require.NoError(t, err)
	require.NotNil(t, signature)
	assert.Equal(t, common.BytesToAddress(mock.Signer), signature.Signer)
}

func TestClient_SignDeleteDataSet(t *testing.T) {
	mock := mockLibforgeSignature()
	service := testutil.RandomIssuer(t)
	alice := testutil.RandomIssuer(t)

	srv := server.NewHTTP(service)
	srv.Handle(sign.DataSetDelete.Command, binding.NewHandler(
		func(req *binding.Request[*sign.DataSetDeleteArguments], res *binding.Response[*sign.DataSetDeleteOK]) error {
			args := req.Task().Arguments()
			assert.Equal(t, "12345", args.DataSet.String())
			return res.SetSuccess(&mock)
		},
	))

	c := newClientBackedByServer(t, srv, service)
	dlg := mustDelegate(t, sign.DataSetDelete, service, alice)

	signature, err := c.SignDeleteDataSet(t.Context(), alice, big.NewInt(12345), []ucan.Delegation{dlg})
	require.NoError(t, err)
	require.NotNil(t, signature)
	assert.Equal(t, common.BytesToAddress(mock.Signer), signature.Signer)
}

func TestClient_ServerError(t *testing.T) {
	service := testutil.RandomIssuer(t)
	alice := testutil.RandomIssuer(t)

	srv := server.NewHTTP(service)
	srv.Handle(sign.DataSetCreate.Command, binding.NewHandler(
		func(req *binding.Request[*sign.DataSetCreateArguments], res *binding.Response[*sign.DataSetCreateOK]) error {
			return res.SetFailure(uerrors.New("Internal", "boom"))
		},
	))

	c := newClientBackedByServer(t, srv, service)
	dlg := mustDelegate(t, sign.DataSetCreate, service, alice)

	_, err := c.SignCreateDataSet(
		t.Context(),
		alice,
		big.NewInt(12345),
		common.HexToAddress("0xabcdef1234567890abcdef1234567890abcdef12"),
		[]eip712.MetadataEntry{},
		[]ucan.Delegation{dlg},
	)
	require.Error(t, err)
}

func mockLibforgeSignature() sign.AuthSignature {
	return sign.AuthSignature{
		Signature:  []byte{0x01, 0x02, 0x03},
		V:          27,
		R:          common.BigToHash(big.NewInt(12345)).Bytes(),
		S:          common.BigToHash(big.NewInt(67890)).Bytes(),
		SignedData: []byte{0xaa, 0xbb},
		Signer:     common.HexToAddress("0x1234567890123456789012345678901234567890").Bytes(),
	}
}
