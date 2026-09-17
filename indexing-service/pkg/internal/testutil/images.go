package testutil

import "os"

// The container images these tests pull, pinned by digest so a test result is
// a function of the commit rather than of the registry. They live here rather
// than inline at the call site so a pin sweep has one place to look per
// module; see MONOREPO_TODO.md on why the Go population cannot be guarded the
// way the compose one is.
//
// Each has an environment override, for pointing at a local build.
const (
	// Our own build of MinIO. Upstream withdrew minio/minio from Docker Hub on
	// 2026-09-11 -- every tag, pinned releases included -- and archived the
	// project, so there is no public image left to pull and no upstream release
	// to track. fil-forge/minio builds this one from source at the tag below;
	// bumping it is a deliberate act there, not a tag that moves underneath us.
	defaultMinioImage = "ghcr.io/fil-forge/minio:RELEASE.2025-10-15T17-29-55Z@sha256:2c4349a1a8dcb3549896109a5363250f77ee90b51f706dc8ceac2b88732a95e7"

	defaultValkeyImage = "valkey/valkey:7.2.5@sha256:57361de39073ea1ef5e7bcfd8068871b21c0a197f01d460e15511f27070fc49b"
)

// MinioImage is the MinIO image the object-store tests run against.
// MINIO_IMAGE overrides it.
func MinioImage() string {
	if img := os.Getenv("MINIO_IMAGE"); img != "" {
		return img
	}
	return defaultMinioImage
}

// ValkeyImage is the Valkey image the Redis-backed store tests run against.
// VALKEY_IMAGE overrides it.
func ValkeyImage() string {
	if img := os.Getenv("VALKEY_IMAGE"); img != "" {
		return img
	}
	return defaultValkeyImage
}
