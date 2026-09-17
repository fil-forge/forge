package aws

import (
	"context"
	crypto_ed25519 "crypto/ed25519"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/fil-forge/forge/indexing-service/pkg/build"
	"github.com/fil-forge/forge/indexing-service/pkg/construct"
	"github.com/fil-forge/forge/indexing-service/pkg/presets"
	"github.com/fil-forge/forge/indexing-service/pkg/redis"
	"github.com/fil-forge/forge/indexing-service/pkg/service/contentclaims"
	"github.com/fil-forge/forge/indexing-service/pkg/telemetry"
	"github.com/fil-forge/forge/indexing-service/pkg/types"
	"github.com/fil-forge/go-ipni-tools/pkg/metadata"
	publisherqueue "github.com/fil-forge/go-ipni-tools/pkg/queue"
	"github.com/fil-forge/go-ipni-tools/pkg/store"
	"github.com/fil-forge/ucantone/did"
	"github.com/fil-forge/ucantone/did/resolver"
	"github.com/fil-forge/ucantone/multikey"
	"github.com/fil-forge/ucantone/multikey/ed25519"
	"github.com/getsentry/sentry-go"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/crypto"
	goredis "github.com/redis/go-redis/v9"
)

// ErrNoPrivateKey means that the value returned from Secrets was empty
var ErrNoPrivateKey = errors.New("no value for private key")

func mustGetEnv(envVar string) string {
	value := os.Getenv(envVar)
	if len(value) == 0 {
		panic(fmt.Errorf("missing env var: %s", envVar))
	}
	return value
}

func mustGetInt(envVar string) int64 {
	stringValue := mustGetEnv(envVar)
	value, err := strconv.ParseInt(stringValue, 10, 64)
	if err != nil {
		panic(fmt.Errorf("parsing env var %s to int: %w", envVar, err))
	}
	return value
}

func mustGetFloat(envVar string) float64 {
	stringValue := mustGetEnv(envVar)
	value, err := strconv.ParseFloat(stringValue, 64)
	if err != nil {
		panic(fmt.Errorf("parsing env var %s to int: %w", envVar, err))
	}
	return value
}

// Config describes all the values required to setup AWS from the environment
type Config struct {
	construct.ServiceConfig
	aws.Config
	ProvidersCacheExpirationSeconds   int64
	NoProvidersCacheExpirationSeconds int64
	ClaimsCacheExpirationSeconds      int64
	IndexesCacheExpirationSeconds     int64
	SQSCachingQueueID                 string
	CachingBucket                     string
	SQSPublishingQueueID              string
	PublishingBucket                  string
	SQSAdvertisementPublishingQueueID string
	ChunkLinksTableName               string
	MetadataTableName                 string
	IPNIStoreBucket                   string
	IPNIStorePrefix                   string
	IPNIAnnounceURLs                  []url.URL
	NotifierHeadBucket                string
	NotifierTopicArn                  string
	ClaimStoreBucket                  string
	ClaimStorePrefix                  string
	BaseTraceSampleRatio              float64
	SentryDSN                         string
	SentryEnvironment                 string
	TelemetryEnabled                  bool
	PrincipalMapping                  map[string]string
	IPNIFormatPeerID                  string
	IPNIFormatEndpoint                string
	Issuer                            multikey.Issuer
}

// FromEnv constructs the AWS Configuration from the environment
func FromEnv(ctx context.Context) Config {
	awsConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic(fmt.Errorf("loading aws default config: %w", err))
	}

	idSigner, err := ed25519.Parse(mustGetEnv("PRIVATE_KEY"))
	if err != nil {
		panic(fmt.Errorf("parsing private key: %s", err))
	}

	var id multikey.Issuer
	id = multikey.KeyIssuer(idSigner)
	if len(os.Getenv("DID")) != 0 {
		d, err := did.Parse(os.Getenv("DID"))
		if err != nil {
			panic(fmt.Errorf("parsing DID: %w", err))
		}
		id = multikey.NewIssuer(d, idSigner)
	}

	// id.Raw() returns the 32-byte seed; libp2p's UnmarshalEd25519PrivateKey
	// wants the 64-byte stdlib form (seed||pub). Expand via NewKeyFromSeed.
	cryptoPrivKey, err := crypto.UnmarshalEd25519PrivateKey(crypto_ed25519.NewKeyFromSeed(idSigner.Raw()))
	if err != nil {
		panic(fmt.Errorf("unmarshaling private key: %w", err))
	}

	ipniStoreKeyPrefix := os.Getenv("IPNI_STORE_KEY_PREFIX")
	if len(ipniStoreKeyPrefix) == 0 {
		ipniStoreKeyPrefix = "ipni/v1/ad/"
	}

	ipniPublisherAnnounceAddress := fmt.Sprintf("/dns/%s/https", mustGetEnv("IPNI_STORE_BUCKET_REGIONAL_DOMAIN"))

	var principalMapping map[string]string
	if os.Getenv("PRINCIPAL_MAPPING") != "" {
		principalMapping = map[string]string{}
		maps.Copy(principalMapping, presets.PrincipalMapping)
		var pm map[string]string
		err := json.Unmarshal([]byte(os.Getenv("PRINCIPAL_MAPPING")), &pm)
		if err != nil {
			panic(fmt.Errorf("parsing principal mapping: %w", err))
		}
		maps.Copy(principalMapping, pm)
	} else {
		principalMapping = presets.PrincipalMapping
	}

	ipniFindURL := os.Getenv("IPNI_ENDPOINT")
	if ipniFindURL == "" {
		ipniFindURL = presets.IPNIFindURL
	}

	var ipniFindFallbackURLs []string
	if os.Getenv("IPNI_FALLBACK_ENDPOINTS") != "" {
		err := json.Unmarshal([]byte(os.Getenv("IPNI_FALLBACK_ENDPOINTS")), &ipniFindFallbackURLs)
		if err != nil {
			panic(fmt.Errorf("parsing IPNI fallback endpoints JSON: %w", err))
		}
	}

	var ipniPublisherDirectAnnounceURLs []string
	if os.Getenv("IPNI_ANNOUNCE_URLS") != "" {
		err := json.Unmarshal([]byte(os.Getenv("IPNI_ANNOUNCE_URLS")), &ipniPublisherDirectAnnounceURLs)
		if err != nil {
			panic(fmt.Errorf("parsing IPNI announce URLs JSON: %w", err))
		}
	} else {
		ipniPublisherDirectAnnounceURLs = presets.IPNIAnnounceURLs
	}

	return Config{
		Config: awsConfig,
		Issuer: id,
		ServiceConfig: construct.ServiceConfig{
			ID:         id,
			PrivateKey: cryptoPrivKey,
			PublicURL:  strings.Split(mustGetEnv("PUBLIC_URL"), ","),
			ProvidersRedis: goredis.ClusterOptions{
				Addrs:                      []string{mustGetEnv("PROVIDERS_CACHE_URL")},
				ReadOnly:                   true,
				RouteRandomly:              true,
				CredentialsProviderContext: redisCredentialVerifier(awsConfig, mustGetEnv("CACHE_USER_ID"), mustGetEnv("PROVIDERS_CACHE_ID")),
				TLSConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
				},
			},
			NoProviderRedis: goredis.ClusterOptions{
				Addrs:                      []string{mustGetEnv("NO_PROVIDERS_CACHE_URL")},
				ReadOnly:                   true,
				RouteRandomly:              true,
				CredentialsProviderContext: redisCredentialVerifier(awsConfig, mustGetEnv("CACHE_USER_ID"), mustGetEnv("NO_PROVIDERS_CACHE_ID")),
				TLSConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
				},
			},
			ClaimsRedis: goredis.ClusterOptions{
				Addrs:                      []string{mustGetEnv("CLAIMS_CACHE_URL")},
				ReadOnly:                   true,
				RouteRandomly:              true,
				CredentialsProviderContext: redisCredentialVerifier(awsConfig, mustGetEnv("CACHE_USER_ID"), mustGetEnv("CLAIMS_CACHE_ID")),
				TLSConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
				},
			},
			IndexesRedis: goredis.ClusterOptions{
				Addrs:                      []string{mustGetEnv("INDEXES_CACHE_URL")},
				ReadOnly:                   true,
				RouteRandomly:              true,
				CredentialsProviderContext: redisCredentialVerifier(awsConfig, mustGetEnv("CACHE_USER_ID"), mustGetEnv("INDEXES_CACHE_ID")),
				TLSConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
				},
			},
			IPNIFindURL:            ipniFindURL,
			IPNIFindFallbackURLs:   ipniFindFallbackURLs,
			IPNIAnnounceAddrs:      []string{ipniPublisherAnnounceAddress},
			IPNIDirectAnnounceURLs: ipniPublisherDirectAnnounceURLs,
		},
		ProvidersCacheExpirationSeconds:   mustGetInt("PROVIDERS_CACHE_EXPIRATION_SECONDS"),
		NoProvidersCacheExpirationSeconds: mustGetInt("NO_PROVIDERS_CACHE_EXPIRATION_SECONDS"),
		ClaimsCacheExpirationSeconds:      mustGetInt("CLAIMS_CACHE_EXPIRATION_SECONDS"),
		IndexesCacheExpirationSeconds:     mustGetInt("INDEXES_CACHE_EXPIRATION_SECONDS"),
		SQSCachingQueueID:                 mustGetEnv("PROVIDER_CACHING_QUEUE_ID"),
		CachingBucket:                     mustGetEnv("PROVIDER_CACHING_BUCKET_NAME"),
		SQSPublishingQueueID:              mustGetEnv("IPNI_PUBLISHER_QUEUE_ID"),
		PublishingBucket:                  mustGetEnv("IPNI_PUBLISHER_BUCKET_NAME"),
		SQSAdvertisementPublishingQueueID: mustGetEnv("IPNI_ADVERTISEMENT_PUBLISHING_QUEUE_ID"),
		ChunkLinksTableName:               mustGetEnv("CHUNK_LINKS_TABLE_ID"),
		MetadataTableName:                 mustGetEnv("METADATA_TABLE_ID"),
		IPNIStoreBucket:                   mustGetEnv("IPNI_STORE_BUCKET_NAME"),
		IPNIStorePrefix:                   ipniStoreKeyPrefix,
		NotifierHeadBucket:                mustGetEnv("NOTIFIER_HEAD_BUCKET_NAME"),
		ClaimStoreBucket:                  mustGetEnv("CLAIM_STORE_BUCKET_NAME"),
		ClaimStorePrefix:                  os.Getenv("CLAIM_STORE_KEY_PREFIX"),
		BaseTraceSampleRatio:              mustGetFloat("BASE_TRACE_SAMPLE_RATIO"),
		SentryDSN:                         os.Getenv("SENTRY_DSN"),
		SentryEnvironment:                 os.Getenv("SENTRY_ENVIRONMENT"),
		TelemetryEnabled:                  os.Getenv("TELEMETRY_DISABLED") == "",
		IPNIFormatPeerID:                  os.Getenv("IPNI_FORMAT_PEER_ID"),
		IPNIFormatEndpoint:                os.Getenv("IPNI_FORMAT_ENDPOINT"),
		PrincipalMapping:                  principalMapping,
	}
}

// Construct constructs types.Service from AWS deps for Lamda functions
func Construct(cfg Config) (types.Service, error) {
	httpClient := construct.DefaultHTTPClient()
	providersClient := goredis.NewClusterClient(&cfg.ProvidersRedis)
	noProvidersClient := goredis.NewClusterClient(&cfg.NoProviderRedis)
	claimsClient := goredis.NewClusterClient(&cfg.ClaimsRedis)
	indexesClient := goredis.NewClusterClient(&cfg.IndexesRedis)

	// instrument HTTP and redis clients if telemetry is enabled
	if cfg.TelemetryEnabled {
		httpClient = telemetry.InstrumentHTTPClient(construct.DefaultHTTPClient())
		providersClient = telemetry.InstrumentRedisClient(providersClient)
		noProvidersClient = telemetry.InstrumentRedisClient(noProvidersClient)
		claimsClient = telemetry.InstrumentRedisClient(claimsClient)
		indexesClient = telemetry.InstrumentRedisClient(indexesClient)
	}

	cachingQueue := NewSQSCachingQueue(cfg.Config, cfg.SQSCachingQueueID, cfg.CachingBucket)
	ipniStore := NewS3Store(cfg.Config, cfg.IPNIStoreBucket, cfg.IPNIStorePrefix)
	claimBucketStore := contentclaims.NewStoreFromBucket(NewS3Store(cfg.Config, cfg.ClaimStoreBucket, cfg.ClaimStorePrefix))
	chunkLinksTable := NewDynamoProviderContextTable(cfg.Config, cfg.ChunkLinksTableName)
	metadataTable := NewDynamoProviderContextTable(cfg.Config, cfg.MetadataTableName)
	publisherStore := store.NewPublisherStore(ipniStore, chunkLinksTable, metadataTable, store.WithMetadataContext(metadata.MetadataContext))

	publishingQueue := NewSQSPublishingQueue(cfg.Config, cfg.SQSPublishingQueueID, cfg.PublishingBucket)
	queuePublisher := publisherqueue.NewQueuePublisher(publishingQueue)
	var provIndexLog logging.EventLogger
	if cfg.SentryDSN != "" && cfg.SentryEnvironment != "" {
		err := sentry.Init(sentry.ClientOptions{
			Dsn:           cfg.SentryDSN,
			Environment:   cfg.SentryEnvironment,
			Release:       build.Version,
			Transport:     sentry.NewHTTPSyncTransport(),
			EnableTracing: false,
		})
		if err != nil {
			return nil, fmt.Errorf("initializing sentry: %w", err)
		}
		provIndexLog = telemetry.NewSentryLogger("providerindex")
	}
	opts := []construct.Option{
		construct.SkipNotification(),
		construct.WithCachingQueue(cachingQueue),
		construct.WithPublisherStore(publisherStore),
		construct.WithAsyncPublisher(queuePublisher),
		construct.WithStartIPNIServer(false),
		construct.WithClaimsStore(claimBucketStore),
		construct.WithHTTPClient(httpClient),
		construct.WithProvidersClient(redis.NewClusterClientAdapter(providersClient)),
		construct.WithNoProvidersClient(noProvidersClient),
		construct.WithClaimsClient(claimsClient),
		construct.WithIndexesClient(indexesClient),
		construct.WithProvidersCacheOptions(redis.ExpirationTime(time.Duration(cfg.ProvidersCacheExpirationSeconds) * time.Second)),
		construct.WithNoProvidersCacheOptions(redis.ExpirationTime(time.Duration(cfg.NoProvidersCacheExpirationSeconds) * time.Second)),
		construct.WithClaimsCacheOptions(redis.ExpirationTime(time.Duration(cfg.ClaimsCacheExpirationSeconds) * time.Second)),
		construct.WithIndexesCacheOptions(redis.ExpirationTime(time.Duration(cfg.IndexesCacheExpirationSeconds) * time.Second)),
		construct.WithProviderIndexLogger(provIndexLog),
	}

	service, err := construct.Construct(
		cfg.ServiceConfig,
		opts...,
	)
	if err != nil {
		return nil, err
	}

	return service, nil
}

func NewPrincipalMappingResolver(mapping map[string]string) (resolver.WellKnown, error) {
	resolver := resolver.WellKnown{}
	for didStr, verifierStr := range mapping {
		d, err := did.Parse(didStr)
		if err != nil {
			return nil, fmt.Errorf("parsing DID %q for principal mapping: %w", didStr, err)
		}

		// Deprecated
		if verifierStr[:8] == "did:key:" {
			verifierStr = verifierStr[8:]
		}

		ver, err := multikey.Parse(verifierStr)
		if err != nil {
			return nil, fmt.Errorf("parsing verifier %q for principal mapping: %w", verifierStr, err)
		}

		doc := did.NewDocument(d)
		vm := multikey.DeriveVerificationMethod(doc.Fragment("key"), ver)

		if err := doc.VerificationMethods.Add(vm); err != nil {
			return nil, err
		}
		if err := doc.CapabilityDelegation.Add(vm); err != nil {
			return nil, err
		}
		if err := doc.CapabilityInvocation.Add(vm); err != nil {
			return nil, err
		}

		resolver[d] = doc
	}
	return resolver, nil
}
