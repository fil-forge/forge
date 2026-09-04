package main

import (
	"bytes"
	"context"
	"fmt"
	"net/url"

	"github.com/fil-forge/forge/protocol/blobindex"
	assertcaps "github.com/fil-forge/forge/protocol/commands/assert"
	"github.com/fil-forge/forge/protocol/digestutil"
	"github.com/fil-forge/ucantone/did"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/fil-forge/ucantone/ucan/container"
	"github.com/fil-forge/ucantone/ucan/invocation"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multibase"
	"github.com/multiformats/go-multihash"
	"github.com/urfave/cli/v2"

	"github.com/fil-forge/forge/indexing-service/pkg/client"
	"github.com/fil-forge/forge/indexing-service/pkg/telemetry"
	"github.com/fil-forge/forge/indexing-service/pkg/types"
)

var queryCmd = &cli.Command{
	Name:  "query",
	Usage: "query an indexing server and print out the results",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "url",
			Aliases: []string{"u"},
			Value:   "https://indexer.forge.fil.one",
			Usage:   "URL of the indexer to query.",
		},
		&cli.StringFlag{
			Name:    "space",
			Aliases: []string{"s"},
			Usage:   "DID of a space to filter results by.",
		},
		&cli.StringFlag{
			Name:    "type",
			Aliases: []string{"t"},
			Usage:   "type of query to perform ['standard' | 'location' | 'index_or_location']",
			Value:   "standard",
		},
		&cli.StringFlag{
			Name:    "delegations",
			Aliases: []string{"d"},
			Usage:   "a UCAN container of delegations allowing the indexer to fetch content from the space",
		},
		&cli.BoolFlag{
			Name:    "enabled-telemetry",
			Usage:   "propagate tracing context on query requests",
			EnvVars: []string{"INDEXER_CLIENT_TELEMETRY_ENABLED"},
		},
	},
	Action: func(cCtx *cli.Context) error {
		serviceURL, err := url.Parse(cCtx.String("url"))
		if err != nil {
			return fmt.Errorf("parsing service URL: %w", err)
		}

		serviceDID, err := did.Parse(fmt.Sprintf("did:web:%s", serviceURL.Hostname()))
		if err != nil {
			return fmt.Errorf("parsing service DID: %w", err)
		}

		// if telemetry is enabled for this query, setup the provider
		type otelCloser = func(context.Context) error
		var otelClose otelCloser
		if cCtx.Bool("telemetry") {
			otelClose, err = telemetry.SetupClientTelemetry(cCtx.Context)
			if err != nil {
				return fmt.Errorf("setting up telemetry: %w", err)
			}
		}

		c, err := client.New(serviceDID, *serviceURL, client.WithTelemetryEnabled(cCtx.Bool("telemetry")))
		if err != nil {
			return fmt.Errorf("creating client: %w", err)
		}

		var cids []cid.Cid
		for _, arg := range cCtx.Args().Slice() {
			cid, err := parseCID(arg)
			if err != nil {
				return fmt.Errorf("parsing CID/multihash: %w", err)
			}
			cids = append(cids, cid)
		}
		if len(cids) == 0 {
			return fmt.Errorf("missing CID/multihash for query: %w", err)
		}

		var digests []multihash.Multihash
		for _, cid := range cids {
			digests = append(digests, cid.Hash())
		}

		var spaces []did.DID
		if cCtx.IsSet("space") {
			space, err := did.Parse(cCtx.String("space"))
			if err != nil {
				return fmt.Errorf("parsing space DID: %w", err)
			}
			spaces = append(spaces, space)
		}

		queryType := types.QueryTypeStandard
		if cCtx.IsSet("type") {
			queryType, err = types.ParseQueryType(cCtx.String("type"))
			if err != nil {
				return fmt.Errorf("error in query type: %w", err)
			}
		}

		var delegations []ucan.Delegation
		if cCtx.IsSet("delegations") {
			ct, err := container.Decode([]byte(cCtx.String("delegations")))
			if err != nil {
				return fmt.Errorf("parsing UCAN container: %w", err)
			}
			delegations = ct.Delegations()
		}

		qr, err := c.QueryClaims(cCtx.Context, types.Query{
			Type:        queryType,
			Hashes:      digests,
			Match:       types.Match{Subject: spaces},
			Delegations: delegations,
		})
		if err != nil {
			return fmt.Errorf("querying service: %w", err)
		}

		blockmap := map[cid.Cid][]byte{}
		for _, b := range qr.Blocks() {
			blockmap[b.Link] = b.Data
		}

		fmt.Println("")
		fmt.Println("Query:")
		fmt.Printf("  Hashes (%d):\n", len(digests))
		for _, digest := range digests {
			fmt.Printf("    %s\n", digestutil.Format(digest))
		}
		if len(spaces) > 0 {
			fmt.Printf("  Spaces (%d):\n", len(spaces))
			for _, space := range spaces {
				fmt.Printf("    %s\n", space.String())
			}
		}
		fmt.Println("")
		fmt.Println("Results:")
		fmt.Printf("  Claims (%d):\n", len(qr.Claims()))
		for _, root := range qr.Claims() {
			claimBytes, ok := blockmap[root]
			if !ok {
				return fmt.Errorf("missing claim block: %w", err)
			}
			claim, err := invocation.Decode(claimBytes)
			if err != nil {
				return fmt.Errorf("decoding claim: %w", err)
			}

			fmt.Printf("    %s\n", claim.Link())
			fmt.Println("      Type:")
			fmt.Printf("        %s\n", claim.Command())
			switch claim.Command() {
			case assertcaps.Location.Command:
				var args assertcaps.LocationArguments
				if err := args.UnmarshalCBOR(bytes.NewReader(claim.ArgumentsBytes())); err != nil {
					return fmt.Errorf("decoding %s arguments: %w", assertcaps.Location, err)
				}
				fmt.Println("      Content:")
				fmt.Printf("        %s\n", digestutil.Format(args.Content))
				if args.Space != did.Undef {
					fmt.Println("      Space:")
					fmt.Printf("        %s\n", args.Space)
				}
				fmt.Println("      Locations:")
				for _, location := range args.Location {
					fmt.Printf("        %s\n", location.URL())
				}
				if args.Range != nil {
					fmt.Printf("      Range: %d-", args.Range.Start)
					if args.Range.End != nil {
						fmt.Printf("%d\n", *args.Range.End)
					} else {
						fmt.Println("")
					}
				}
			case assertcaps.Equals.Command:
				var args assertcaps.EqualsArguments
				if err := args.UnmarshalCBOR(bytes.NewReader(claim.ArgumentsBytes())); err != nil {
					return fmt.Errorf("decoding %s arguments: %w", assertcaps.Equals, err)
				}
				fmt.Println("      Content:")
				fmt.Printf("        %s\n", digestutil.Format(args.Content))
				fmt.Println("      Equals:")
				fmt.Printf("        %s\n", args.Equals)
			case assertcaps.Index.Command:
				var args assertcaps.IndexArguments
				if err := args.UnmarshalCBOR(bytes.NewReader(claim.ArgumentsBytes())); err != nil {
					return fmt.Errorf("decoding %s arguments: %w", assertcaps.Index.Command, err)
				}
				fmt.Println("      Index:")
				fmt.Printf("        %s\n", args.Index)
			default:
				fmt.Println("      (Unknown Claim)")
			}
		}

		fmt.Println("")
		fmt.Printf("  Indexes (%d):\n", len(qr.Indexes()))
		for _, root := range qr.Indexes() {
			indexBytes, ok := blockmap[root]
			if !ok {
				return fmt.Errorf("missing index block: %w", err)
			}
			index, err := blobindex.Extract(bytes.NewReader(indexBytes))
			if err != nil {
				return fmt.Errorf("decoding index: %w", err)
			}

			fmt.Printf("    %s\n", root)
			fmt.Printf("      Shards (%d):\n", index.Shards().Size())
			for shard, slices := range index.Shards().Iterator() {
				fmt.Printf("        %s\n", digestutil.Format(shard))
				fmt.Printf("          Slices (%d):\n", slices.Size())
				for digest, position := range slices.Iterator() {
					fmt.Printf("            %s @ %d-%d\n", digestutil.Format(digest), position.Start, position.End)
				}
			}
		}

		if otelClose != nil {
			if err := otelClose(cCtx.Context); err != nil {
				log.Warnf("failed to close telemetry provider: %s", err)
			}
		}
		return nil
	},
}

func parseCID(input string) (cid.Cid, error) {
	c, err := cid.Parse(input)
	if err == nil {
		return c, nil
	}

	_, b, err := multibase.Decode(input)
	if err != nil {
		return cid.Undef, err
	}

	_, digest, err := multihash.MHFromBytes(b)
	if err != nil {
		return cid.Undef, err
	}

	return cid.NewCidV1(cid.Raw, digest), nil
}
