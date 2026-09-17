package contentclaims

import (
	"github.com/fil-forge/indexing-service/pkg/types"
	assertcaps "github.com/fil-forge/libforge/commands/assert"
	claimcaps "github.com/fil-forge/libforge/commands/claim"
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/did"
	"github.com/fil-forge/ucantone/multikey/ed25519/verifier"
	"github.com/fil-forge/ucantone/server"
	"github.com/fil-forge/ucantone/ucan"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

var log = logging.Logger("contentclaims")

func NewUCANService(service types.Publisher) []server.Route {
	return []server.Route{
		assertcaps.Equals.Route(
			func(
				req *binding.Request[*assertcaps.EqualsArguments],
				res *binding.Response[*assertcaps.EqualsOK],
			) error {
				claim := req.Invocation()
				err := service.Publish(req.Context(), claim, req.Metadata())
				if err != nil {
					log.Errorf("publishing equals claim: %s", err)
					return err
				}
				return res.SetSuccess(&assertcaps.EqualsOK{})
			},
		),
		assertcaps.Index.Route(
			func(
				req *binding.Request[*assertcaps.IndexArguments],
				res *binding.Response[*assertcaps.IndexOK],
			) error {
				claim := req.Invocation()
				err := service.Publish(req.Context(), claim, req.Metadata())
				if err != nil {
					log.Errorf("publishing index claim: %s", err)
					return err
				}
				return res.SetSuccess(&assertcaps.IndexOK{})
			},
		),
		claimcaps.Cache.Route(
			func(
				req *binding.Request[*claimcaps.CacheArguments],
				res *binding.Response[*claimcaps.CacheOK],
			) error {
				args := req.Task().Arguments()
				peerid, err := toPeerID(req.Invocation().Issuer())
				if err != nil {
					return err
				}

				var addrs []multiaddr.Multiaddr
				for _, a := range args.Provider.Addresses {
					ma, err := multiaddr.NewMultiaddrBytes(a)
					if err != nil {
						return err
					}
					addrs = append(addrs, ma)
				}

				provider := peer.AddrInfo{ID: peerid, Addrs: addrs}

				var claim ucan.Invocation
				for _, inv := range req.Metadata().Invocations() {
					if inv.Link() == args.Claim {
						claim = inv
						break
					}
				}
				if claim == nil {
					return res.SetFailure(ErrMissingClaim)
				}

				err = service.Cache(req.Context(), provider, claim, req.Metadata())
				if err != nil {
					log.Errorf("caching claim: %s", err)
					return err
				}
				return res.SetSuccess(&claimcaps.CacheOK{})
			}),
	}
}

func toPeerID(principal did.DID) (peer.ID, error) {
	vfr, err := verifier.ParseKeyDID(principal.String())
	if err != nil {
		return "", err
	}
	pub, err := crypto.UnmarshalEd25519PublicKey(vfr.Raw())
	if err != nil {
		return "", err
	}
	return peer.IDFromPublicKey(pub)
}
