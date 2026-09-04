package handlers

import (
	"strings"
	"time"

	"github.com/fil-forge/forge/protocol/commands/access"
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/errors"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/fil-forge/ucantone/ucan/container"
	"github.com/fil-forge/ucantone/ucan/delegation"
	"github.com/ipfs/go-cid"
)

// validity is the time a granted delegation is valid for.
const grantValidity = time.Hour

// NewAccessGrantHandler returns a bindexec UCAN handler for /access/grant.
// It issues a one-hour delegation per requested capability, returning the
// CIDs in the receipt OK and attaching the signed delegations to the
// response container as metadata.
func NewAccessGrantHandler(id ucan.Issuer) binding.HandlerFunc[*access.GrantArguments, *access.GrantOK] {
	return func(req *binding.Request[*access.GrantArguments], res *binding.Response[*access.GrantOK]) error {
		args := req.Task().Arguments()
		inv := req.Invocation()

		log.Infow(
			"handling access grant",
			"command", access.GrantCommand,
			"issuer", inv.Issuer(),
			"attenuations", args.Attenuations,
			"cause", args.Cause,
		)

		if len(args.Attenuations) == 0 {
			return res.SetFailure(access.ErrMissingCapability)
		}

		audience := inv.Issuer()
		exp := ucan.UnixTimestamp(time.Now().Add(grantValidity).Unix())

		delegations := make([]ucan.Delegation, 0, len(args.Attenuations))
		links := make([]cid.Cid, 0, len(args.Attenuations))
		for _, ar := range args.Attenuations {
			if !strings.HasPrefix(ar.Command.String(), "/pdp/sign/") {
				return res.SetFailure(errors.New(access.UnknownAbilityErrorName,
					"unknown ability: %s", ar.Command))
			}

			dlg, err := delegation.Delegate(
				id,
				audience,
				id.DID(),
				ar.Command,
				delegation.WithExpiration(exp),
			)
			if err != nil {
				return res.SetFailure(err)
			}
			log.Infow("delegated capability", "command", ar.Command, "audience", audience)
			delegations = append(delegations, dlg)
			links = append(links, dlg.Link())
		}

		// Attach the signed delegation envelopes as response metadata so the
		// caller can recover them from the receipt container.
		if err := res.SetMetadata(container.New(container.WithDelegations(delegations...))); err != nil {
			return err
		}

		return res.SetSuccess(&access.GrantOK{Delegations: links})
	}
}
