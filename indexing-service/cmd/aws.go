package main

import (
	"fmt"
	"time"

	"github.com/fil-forge/go-ipni-tools/pkg/metadata"
	"github.com/fil-forge/go-ipni-tools/pkg/notifier"
	"github.com/fil-forge/go-ipni-tools/pkg/publisher"
	"github.com/fil-forge/go-ipni-tools/pkg/queue"
	"github.com/fil-forge/go-ipni-tools/pkg/store"
	"github.com/fil-forge/indexing-service/pkg/aws"
	"github.com/fil-forge/indexing-service/pkg/redis"
	"github.com/fil-forge/indexing-service/pkg/server"
	"github.com/fil-forge/indexing-service/pkg/service/providercacher"
	"github.com/fil-forge/indexing-service/pkg/service/providerindex/remotesyncer"
	"github.com/fil-forge/indexing-service/pkg/telemetry"
	"github.com/fil-forge/libforge/identity"
	"github.com/fil-forge/ucantone/did/key"
	"github.com/fil-forge/ucantone/did/resolver"
	"github.com/fil-forge/ucantone/did/web"
	userver "github.com/fil-forge/ucantone/server"
	"github.com/fil-forge/ucantone/validator"
	goredis "github.com/redis/go-redis/v9"
	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/otel/sdk/trace"
)

var awsCmd = &cli.Command{
	Name:  "aws",
	Usage: "Run the indexing service as a containerized server in AWS",
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name:    "port",
			Aliases: []string{"p"},
			Value:   8080,
			Usage:   "Port to bind the server to",
		},
	},
	Action: func(cCtx *cli.Context) error {
		addr := fmt.Sprintf(":%d", cCtx.Int("port"))
		cfg := aws.FromEnv(cCtx.Context)
		id := identity.Identity{Issuer: cfg.ID}
		srvOpts := []server.Option{
			server.WithIdentity(id),
		}

		wellKnownResolv, err := aws.NewPrincipalMappingResolver(cfg.PrincipalMapping)
		if err != nil {
			return fmt.Errorf("creating principal mapping resolver: %w", err)
		}

		doc, err := id.DIDDocument()
		if err != nil {
			return fmt.Errorf("creating DID document: %w", err)
		}
		wellKnownResolv[cfg.ID.DID()] = doc

		httpResolv, err := web.NewResolver()
		if err != nil {
			return fmt.Errorf("creating HTTP resolver: %w", err)
		}

		srvOpts = append(
			srvOpts,
			server.WithContentClaimsOptions(
				userver.WithValidationOptions(
					validator.WithDIDResolver(resolver.ByMethod{
						"key": key.Resolver,
						"web": resolver.Tiered{
							wellKnownResolv,
							resolver.NewCached(httpResolv, time.Hour*3),
						},
					}),
				),
			),
		)

		ipniSrvOpts, err := ipniOpts(cfg.IPNIFormatPeerID, cfg.IPNIFormatEndpoint)
		if err != nil {
			return fmt.Errorf("setting up IPNI options: %w", err)
		}
		srvOpts = append(srvOpts, ipniSrvOpts...)

		if cfg.TelemetryEnabled {
			var telemetryOpts []telemetry.TelemetryOption
			if cfg.BaseTraceSampleRatio < 1.0 {
				telemetryOpts = append(telemetryOpts, telemetry.WithBaseSampler(trace.TraceIDRatioBased(cfg.BaseTraceSampleRatio)))
			}
			telemetryShutdown, err := telemetry.SetupTelemetry(cCtx.Context, &cfg.Config, telemetryOpts...)
			if err != nil {
				panic(err)
			}
			defer telemetryShutdown(cCtx.Context)

			srvOpts = append(srvOpts, server.WithTelemetry())
		}

		indexer, err := aws.Construct(cfg)
		if err != nil {
			return err
		}

		notifier, err := setupIPNIPipeline(cfg)
		if err != nil {
			return err
		}
		notifier.Start(cCtx.Context)
		defer notifier.Stop()

		cacher, err := setupProviderCacher(cfg)
		if err != nil {
			return err
		}
		cacher.Start()
		defer cacher.Stop()

		publisher, advertisementPublisher, err := setupIPNIPublisher(cfg)
		if err != nil {
			return err
		}
		publisher.Start()
		defer publisher.Stop()
		advertisementPublisher.Start()
		defer advertisementPublisher.Stop()

		return server.ListenAndServe(addr, indexer, srvOpts...)
	},
}

func setupIPNIPipeline(cfg aws.Config) (*notifier.Notifier, error) {
	// setup remote IPNI syncer
	providersRedis := goredis.NewClusterClient(&cfg.ProvidersRedis)
	if cfg.TelemetryEnabled {
		providersRedis = telemetry.InstrumentRedisClient(providersRedis)
	}
	providerStore := redis.NewProviderStore(redis.NewClusterClientAdapter(providersRedis))
	publisherStore := setupIPNIPublisherStore(cfg)
	remoteSyncer := remotesyncer.New(providerStore, publisherStore)

	// setup notifier to periodically check IPNI and notify remote syncer if updates are required
	headStore := aws.NewS3Store(cfg.Config, cfg.NotifierHeadBucket, "")
	notifier, err := notifier.NewNotifierWithStorage(cfg.IPNIFindURL, cfg.PrivateKey, headStore)
	if err != nil {
		return nil, fmt.Errorf("creating notifier: %w", err)
	}

	notifier.Notify(remoteSyncer.HandleRemoteSync)
	return notifier, nil
}

func setupProviderCacher(cfg aws.Config) (*providercacher.CachingQueuePoller, error) {
	cachingQueue := aws.NewSQSCachingQueue(cfg.Config, cfg.SQSCachingQueueID, cfg.CachingBucket)

	providersRedis := goredis.NewClusterClient(&cfg.ProvidersRedis)
	if cfg.TelemetryEnabled {
		providersRedis = telemetry.InstrumentRedisClient(providersRedis)
	}
	providerStore := redis.NewProviderStore(redis.NewClusterClientAdapter(providersRedis))
	providerCacher := providercacher.NewSimpleProviderCacher(providerStore)

	return providercacher.NewCachingQueuePoller(cachingQueue, providerCacher)
}

func setupIPNIPublisherStore(cfg aws.Config) *store.AdStore {
	ipniStore := aws.NewS3Store(cfg.Config, cfg.IPNIStoreBucket, cfg.IPNIStorePrefix)
	chunkLinksTable := aws.NewDynamoProviderContextTable(cfg.Config, cfg.ChunkLinksTableName)
	metadataTable := aws.NewDynamoProviderContextTable(cfg.Config, cfg.MetadataTableName)
	return store.NewPublisherStore(ipniStore, chunkLinksTable, metadataTable, store.WithMetadataContext(metadata.MetadataContext))
}

func setupIPNIPublisher(cfg aws.Config) (*queue.PublishingQueuePoller, *queue.AdvertisementPublishingQueuePoller, error) {
	publishingQueue := aws.NewSQSPublishingQueue(cfg.Config, cfg.SQSPublishingQueueID, cfg.PublishingBucket)
	advertisementPublishingQueue := aws.NewSQSAdvertisementPublishingQueue(cfg.Config, cfg.SQSAdvertisementPublishingQueueID)
	store := setupIPNIPublisherStore(cfg)
	advertisementQueuePublisher := queue.NewAdvertisementQueuePublisher(advertisementPublishingQueue, store)
	publishingQueuePoller, err := queue.NewPublishingQueuePoller(publishingQueue, advertisementQueuePublisher)
	if err != nil {
		return nil, nil, fmt.Errorf("creating publishing queue poller: %w", err)
	}
	advertisementPublisher, err := publisher.NewAdvertisementPublisher(
		cfg.ServiceConfig.PrivateKey,
		store,
		publisher.WithDirectAnnounce(cfg.ServiceConfig.IPNIDirectAnnounceURLs...),
		publisher.WithAnnounceAddrs(cfg.ServiceConfig.IPNIAnnounceAddrs...),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("creating IPNI advertisement publisher: %w", err)
	}
	advertisementPublishingQueuePoller, err := queue.NewAdvertisementPublishingQueuePoller(advertisementPublishingQueue, advertisementPublisher)
	if err != nil {
		return nil, nil, fmt.Errorf("creating advertisement publishing queue poller: %w", err)
	}
	return publishingQueuePoller, advertisementPublishingQueuePoller, nil
}
