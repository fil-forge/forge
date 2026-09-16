package testutil

import (
	"context"
	"os"
	"testing"

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
const defaultMinioImage = "ghcr.io/fil-forge/minio:RELEASE.2025-10-15T17-29-55Z@sha256:2c4349a1a8dcb3549896109a5363250f77ee90b51f706dc8ceac2b88732a95e7"

func minioImage() string {
	if img := os.Getenv("MINIO_IMAGE"); img != "" {
		return img
	}
	return defaultMinioImage
}

// RunMinioContainer starts a MinIO container and waits until its object layer
// is ready to serve requests.
//
// The testcontainers minio module's default wait strategy polls
// /minio/health/live, which returns 200 as soon as the process accepts
// connections — before the object layer is initialized — so a request issued
// immediately after startup can fail with a transient "Server not
// initialized" error. /minio/health/ready behaves the same way (it only sets
// the x-minio-server-status header). /minio/health/cluster is the endpoint
// that returns 503 until the object layer is up, so wait on that instead.
func RunMinioContainer(ctx context.Context) (*minio.MinioContainer, error) {
	return minio.Run(ctx, minioImage(),
		testcontainers.WithWaitStrategy(
			wait.ForHTTP("/minio/health/cluster").WithPort("9000/tcp"),
		),
	)
}

func StartMinioContainer(t *testing.T) string {
	container, err := RunMinioContainer(t.Context())
	testcontainers.CleanupContainer(t, container)
	require.NoError(t, err)

	endpoint, err := container.ConnectionString(t.Context())
	require.NoError(t, err)

	t.Logf("Minio listening on: http://%s", endpoint)
	return endpoint
}
