package service

import (
	"net/url"
	"testing"

	"github.com/fil-forge/forge/indexing-service/pkg/internal/testutil"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
)

// TODO: TestQuery, TestPublishIndexClaim, TestPublishEqualsClaim, TestCacheClaim
// — the original suites exercised the legacy go-ucanto delegation flow and are
// being rewritten against the ucantone invocation flow.

func TestUrlForResource(t *testing.T) {
	const addrBase = "/dns/forge.fil.one/https/http-path/"
	testCases := []struct {
		name         string
		addrs        []ma.Multiaddr
		expectedUrl  string
		expectErr    bool
		replacements []replacement
	}{
		{
			name: "happy path",
			addrs: []ma.Multiaddr{
				testutil.Must(ma.NewMultiaddr(addrBase + url.PathEscape("/claims/{claim}")))(t),
			},
			expectedUrl: "https://forge.fil.one/claims/123",
			replacements: []replacement{
				{resourcePlaceholder: "{claim}", resourceID: "123"},
			},
		},
		{
			name: "multiple addresses, uses the first one that contains the placeholder",
			addrs: []ma.Multiaddr{
				testutil.Must(ma.NewMultiaddr(addrBase + url.PathEscape("/blobs/{blob}")))(t),
				testutil.Must(ma.NewMultiaddr(addrBase + url.PathEscape("/claims1/{claim}")))(t),
				testutil.Must(ma.NewMultiaddr(addrBase + url.PathEscape("/claims2/{claim}")))(t),
			},
			expectedUrl: "https://forge.fil.one/claims1/123",
			replacements: []replacement{
				{resourcePlaceholder: "{claim}", resourceID: "123"},
			},
		},
		{
			name:      "no addresses in peer addr info",
			addrs:     []ma.Multiaddr{},
			expectErr: true,
			replacements: []replacement{
				{resourcePlaceholder: "{claim}", resourceID: ""},
			},
		},
		{
			name: "no address contains the placeholder",
			addrs: []ma.Multiaddr{
				testutil.Must(ma.NewMultiaddr(addrBase + url.PathEscape("/claims/{foo}")))(t),
				testutil.Must(ma.NewMultiaddr(addrBase + url.PathEscape("/claims/{bar}")))(t),
				testutil.Must(ma.NewMultiaddr(addrBase + url.PathEscape("/claims/{baz}")))(t),
			},
			expectErr: true,
			replacements: []replacement{
				{resourcePlaceholder: "{claim}", resourceID: ""},
			},
		},
		{
			name: "happy path, multiple replacements",
			addrs: []ma.Multiaddr{
				testutil.Must(ma.NewMultiaddr(addrBase + url.PathEscape("/claims/{claim}-{subclaim}")))(t),
			},
			expectedUrl: "https://forge.fil.one/claims/123-456",
			replacements: []replacement{
				{resourcePlaceholder: "{claim}", resourceID: "123"},
				{resourcePlaceholder: "{subclaim}", resourceID: "456"},
			},
		},
		{
			name: "multiple replacements, only one in source string",
			addrs: []ma.Multiaddr{
				testutil.Must(ma.NewMultiaddr(addrBase + url.PathEscape("/claims/{subclaim}")))(t),
			},
			expectedUrl: "https://forge.fil.one/claims/456",
			replacements: []replacement{
				{resourcePlaceholder: "{claim}", resourceID: "123"},
				{resourcePlaceholder: "{subclaim}", resourceID: "456"},
			},
		},
		{
			name: "multiple addresses, multiple replacements, uses the first one that contains any the placeholder",
			addrs: []ma.Multiaddr{
				testutil.Must(ma.NewMultiaddr(addrBase + url.PathEscape("/blobs/{blob}")))(t),
				testutil.Must(ma.NewMultiaddr(addrBase + url.PathEscape("/claims1/{claim}")))(t),
				testutil.Must(ma.NewMultiaddr(addrBase + url.PathEscape("/claims2/{claim}-{subclaim}")))(t),
			},
			expectedUrl: "https://forge.fil.one/claims1/123",
			replacements: []replacement{
				{resourcePlaceholder: "{claim}", resourceID: "123"},
				{resourcePlaceholder: "{subclaim}", resourceID: "456"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := peer.AddrInfo{Addrs: tc.addrs}
			u, err := urlForResource(provider, tc.replacements)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedUrl, u.String())
			}
		})
	}
}
