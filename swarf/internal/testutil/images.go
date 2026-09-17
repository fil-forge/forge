package testutil

import "os"

// The container images these tests pull, pinned by digest so a test result is
// a function of the commit rather than of the registry. They live here rather
// than inline at the call site so a pin sweep has one place to look per
// module; see MONOREPO_TODO.md on why the Go population cannot be guarded the
// way the compose one is.
//
// This digest is the one the rest of the repository already pins for
// postgres:16-alpine -- hilt, piri, sprue and smelt all name it. Agreeing with
// them matters more than being current: two builds of one tag in one
// repository is the failure rule 2 exists to prevent.
const defaultPostgresImage = "postgres:16-alpine@sha256:cf78e76683b9ca8c5733cbbdce6c9262b45b6767934dd0a95e671f9a0fc20685"

// PostgresImage is the PostgreSQL image the store tests run against.
// POSTGRES_IMAGE overrides it, for pointing at a local build.
func PostgresImage() string {
	if img := os.Getenv("POSTGRES_IMAGE"); img != "" {
		return img
	}
	return defaultPostgresImage
}
