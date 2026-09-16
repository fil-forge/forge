package main

import (
	"context"
	"encoding/json"

	"github.com/aws/aws-lambda-go/events"
	"github.com/fil-forge/forge/indexing-service/cmd/lambda"
	"github.com/fil-forge/forge/indexing-service/pkg/aws"
	"github.com/fil-forge/forge/indexing-service/pkg/redis"
	"github.com/fil-forge/forge/indexing-service/pkg/service/providerindex/remotesyncer"
	"github.com/fil-forge/forge/indexing-service/pkg/telemetry"
	"github.com/fil-forge/go-ipni-tools/pkg/metadata"
	"github.com/fil-forge/go-ipni-tools/pkg/store"
	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	goredis "github.com/redis/go-redis/v9"
)

func main() {
	lambda.Start(makeHandler)
}

func makeHandler(cfg aws.Config) any {
	providersRedis := goredis.NewClusterClient(&cfg.ProvidersRedis)
	if cfg.TelemetryEnabled {
		providersRedis = telemetry.InstrumentRedisClient(providersRedis)
	}
	providerStore := redis.NewProviderStore(redis.NewClusterClientAdapter(providersRedis))
	ipniStore := aws.NewS3Store(cfg.Config, cfg.IPNIStoreBucket, cfg.IPNIStorePrefix)
	chunkLinksTable := aws.NewDynamoProviderContextTable(cfg.Config, cfg.ChunkLinksTableName)
	metadataTable := aws.NewDynamoProviderContextTable(cfg.Config, cfg.MetadataTableName)
	publisherStore := store.NewPublisherStore(ipniStore, chunkLinksTable, metadataTable, store.WithMetadataContext(metadata.MetadataContext))
	remoteSyncer := remotesyncer.New(providerStore, publisherStore)

	return func(ctx context.Context, snsEvent events.SNSEvent) error {
		for _, record := range snsEvent.Records {
			snsRecord := record.SNS
			var snsRemoteSyncMessage aws.SNSRemoteSyncMessage
			err := json.Unmarshal([]byte(snsRecord.Message), &snsRemoteSyncMessage)
			if err != nil {
				return err
			}
			headCid, err := cid.Parse(snsRemoteSyncMessage.Head)
			if err != nil {
				return err
			}
			head := cidlink.Link{Cid: headCid}
			prevCid, err := cid.Parse(snsRemoteSyncMessage.Prev)
			if err != nil {
				return err
			}
			prev := cidlink.Link{Cid: prevCid}
			remoteSyncer.HandleRemoteSync(ctx, head, prev)
		}
		return nil
	}
}
