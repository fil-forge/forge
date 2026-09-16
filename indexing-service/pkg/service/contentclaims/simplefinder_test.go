package contentclaims_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/fil-forge/forge/indexing-service/pkg/internal/testutil"
	"github.com/fil-forge/forge/indexing-service/pkg/service/contentclaims"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/stretchr/testify/require"
)

func TestSimpleFinder__Find(t *testing.T) {
	claim := testutil.RandomIndexClaim(t)
	otherClaim := testutil.RandomIndexClaim(t)

	testCases := []struct {
		name          string
		handler       http.HandlerFunc
		expectedErr   error
		expectedClaim ucan.Invocation
	}{
		{
			name: "success fetch",
			handler: func(w http.ResponseWriter, r *http.Request) {
				testutil.Must(w.Write(claim.Bytes()))(t)
			},
			expectedClaim: claim,
		},
		{
			name: "CID match failure",
			handler: func(w http.ResponseWriter, r *http.Request) {
				testutil.Must(w.Write(otherClaim.Bytes()))(t)
			},
			expectedErr: fmt.Errorf("received claim: %s, does not match expected claim: %s", otherClaim.Link(), claim.Link()),
		},
		{
			name:        "failure",
			handler:     http.NotFound,
			expectedErr: errors.New("failed to unmarshal claim: unmarshaling invocation envelope CBOR: cbor input should be of type array"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testServer := httptest.NewServer(tc.handler)
			defer func() { testServer.Close() }()
			cl := contentclaims.NewSimpleFinder(testServer.Client())
			got, err := cl.Find(context.Background(), claim.Link(), testutil.Must(url.Parse(testServer.URL))(t))
			if tc.expectedErr != nil {
				require.EqualError(t, err, strings.ReplaceAll(tc.expectedErr.Error(), "{url}", testServer.URL))
			} else {
				require.NoError(t, err)
			}
			testutil.RequireEqualClaim(t, tc.expectedClaim, got)
		})
	}
}
