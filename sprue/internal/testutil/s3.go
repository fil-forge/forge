package testutil

import (
	"net/url"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/minio"
	"github.com/testcontainers/testcontainers-go/wait"
)

// minioImage is our own build of MinIO. Upstream withdrew minio/minio from
// Docker Hub on 2026-09-11 -- every tag, pinned releases included -- and
// archived the project, so there is no public image left to pull and no
// upstream release to track. fil-forge/minio builds this one from source at
// the tag below; bumping it is a deliberate act there, not a tag that moves
// underneath us.
//
// MINIO_IMAGE overrides it, for pointing at a local build.
const defaultMinioImage = "ghcr.io/fil-forge/minio:RELEASE.2025-10-15T17-29-55Z"

func minioImage() string {
	if img := os.Getenv("MINIO_IMAGE"); img != "" {
		return img
	}
	return defaultMinioImage
}

func CreateS3(t *testing.T) *url.URL {
	// Wait on /minio/health/cluster, not the module default /minio/health/live:
	// live returns 200 as soon as the process accepts connections, before the
	// object layer is up, so a store created immediately after this returns can
	// fail with a transient "Server not initialized". piri's testutil documents
	// the same reasoning.
	container, err := minio.Run(t.Context(), minioImage(),
		testcontainers.WithWaitStrategy(
			wait.ForHTTP("/minio/health/cluster").WithPort("9000/tcp"),
		),
	)
	testcontainers.CleanupContainer(t, container)
	require.NoError(t, err)

	endpoint, err := container.ConnectionString(t.Context())
	require.NoError(t, err)

	t.Logf("S3 listening on: http://%s", endpoint)
	return Must(url.Parse("http://" + endpoint))(t)
}

func NewS3Client(t *testing.T, endpoint *url.URL) *s3.Client {
	cfg, err := config.LoadDefaultConfig(
		t.Context(),
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID:     "minioadmin",
				SecretAccessKey: "minioadmin",
			},
		}),
		func(o *config.LoadOptions) error {
			o.Region = "us-east-1"
			return nil
		},
	)
	require.NoError(t, err)

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		base := endpoint.String()
		o.BaseEndpoint = &base
		o.UsePathStyle = true
	})
}
