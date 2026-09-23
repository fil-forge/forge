package stack

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/fil-forge/forge/smelt/pkg/manifest"
)

// PiriNodeConfig configures a single piri node in the test stack.
type PiriNodeConfig struct {
	Postgres bool // Use PostgreSQL for this node
	S3       bool // Use S3 for this node
}

// config holds the configuration for a Stack.
type config struct {
	// Image overrides
	piriImage       string
	guppyImage      string
	indexerImage    string
	delegatorImage  string
	uploadImage     string
	hiltImage       string
	signerImage     string
	blockchainImage string
	ipniImage       string
	ingotImage      string
	swarfImage      string
	minioImage      string
	plcImage        string

	// Binary injection: bind-mount host-built binaries over the published
	// images instead of rebuilding the image. serviceBinaries holds explicit
	// prebuilt binaries keyed by smelt service name; workspaceBinaries enables
	// auto-detect-and-build of services from the active go.work use-list.
	serviceBinaries   map[string]string
	workspaceBinaries bool
	// workspaceExclude names services that workspaceBinaries must NOT build,
	// so a pinned image can stand in for them (see WithWorkspaceBinariesExcept).
	workspaceExclude map[string]bool

	// Config injection: bind-mount test-provided config files over a service's
	// in-container config path, keyed by smelt service name.
	serviceConfigs map[string]string

	// Piri node topology. When nil, a single default node is used.
	piriNodes []PiriNodeConfig

	// Snapshot restore. When set, the stack boots from a saved snapshot's
	// keys/proofs/chain-state/volumes, skipping contract deploy and piri
	// registration. Topology comes from the snapshot's embedded smelt.yml;
	// WithPiriCount / WithPiriNodes are rejected when this is set.
	// Exactly one of snapshotPath / embeddedSnapshotName may be set.
	snapshotPath         string
	embeddedSnapshotName string

	// Stack configuration
	timeout       time.Duration
	keepOnFailure bool
}

func defaultConfig() *config {
	return &config{
		timeout: 5 * time.Minute, // Default 5 minute timeout for stack startup
	}
}

// imageOverrides reports the image overrides in force, as "VAR=value" lines
// sorted for stable output. Derived from buildEnv rather than restated, so it
// cannot list a different set from the one the stack actually uses.
func (c *config) imageOverrides() []string {
	var out []string
	for k, v := range c.buildEnv() {
		if strings.HasSuffix(k, "_IMAGE") && v != "" {
			out = append(out, k+"="+v)
		}
	}
	sort.Strings(out)
	return out
}

// buildEnv returns the environment variables for docker-compose.
func (c *config) buildEnv() map[string]string {
	env := make(map[string]string)

	if c.piriImage != "" {
		env["PIRI_IMAGE"] = c.piriImage
	}
	if c.guppyImage != "" {
		env["GUPPY_IMAGE"] = c.guppyImage
	}
	if c.indexerImage != "" {
		env["INDEXER_IMAGE"] = c.indexerImage
	}
	if c.delegatorImage != "" {
		env["DELEGATOR_IMAGE"] = c.delegatorImage
	}
	if c.uploadImage != "" {
		env["UPLOAD_IMAGE"] = c.uploadImage
	}
	if c.hiltImage != "" {
		env["HILT_IMAGE"] = c.hiltImage
	}
	if c.signerImage != "" {
		env["SIGNER_IMAGE"] = c.signerImage
	}
	if c.blockchainImage != "" {
		env["BLOCKCHAIN_IMAGE"] = c.blockchainImage
	}
	if c.ipniImage != "" {
		env["IPNI_IMAGE"] = c.ipniImage
	}
	if c.ingotImage != "" {
		env["INGOT_IMAGE"] = c.ingotImage
	}
	if c.swarfImage != "" {
		env["SWARF_IMAGE"] = c.swarfImage
	}
	if c.minioImage != "" {
		env["MINIO_IMAGE"] = c.minioImage
	}
	if c.plcImage != "" {
		env["PLC_IMAGE"] = c.plcImage
	}

	return env
}

// resolveNodes resolves the piri node configuration into manifest.ResolvedPiriNode list.
func (c *config) resolveNodes() []manifest.ResolvedPiriNode {
	nodes := c.piriNodes
	if nodes == nil {
		nodes = []PiriNodeConfig{{}}
	}

	resolved := make([]manifest.ResolvedPiriNode, len(nodes))
	for i, n := range nodes {
		db := manifest.DBSQLite
		if n.Postgres {
			db = manifest.DBPostgres
		}
		blob := manifest.BlobFS
		if n.S3 {
			blob = manifest.BlobS3
		}
		resolved[i] = manifest.ResolvedPiriNode{
			Name:  fmt.Sprintf("piri-%d", i),
			Index: i,
			Storage: manifest.StorageSpec{
				DB:   db,
				Blob: blob,
			},
		}
	}
	return resolved
}

// Option configures a Stack.
type Option func(*config)

// WithPiriImage sets the piri container image.
func WithPiriImage(image string) Option {
	return func(c *config) {
		c.piriImage = image
	}
}

// WithServiceBinary mounts a prebuilt binary over a service's binary in the
// container, replacing the image's copy without rebuilding the image. The
// binary must be a static linux/amd64 build. service is a smelt service name
// (piri, upload, signing-service, indexer, delegator, guppy).
//
// For building from a local checkout via the Go workspace, prefer
// WithWorkspaceBinaries; this option is the explicit, prebuilt-binary escape
// hatch.
func WithServiceBinary(service, path string) Option {
	return func(c *config) {
		if c.serviceBinaries == nil {
			c.serviceBinaries = map[string]string{}
		}
		c.serviceBinaries[service] = path
	}
}

// WithPiriBinary mounts a local piri binary into the piri container(s),
// replacing the image's binary for fast iteration. The binary must be compiled
// for linux/amd64. Equivalent to WithServiceBinary("piri", path).
func WithPiriBinary(path string) Option {
	return WithServiceBinary("piri", path)
}

// WithServiceConfig mounts a test-provided config file over a service's
// in-container config path (e.g. ingot's /etc/ingot/config.yaml), replacing
// the default that ships with smelt's system definition. This lets a service
// repo's e2e tests exercise config changes without a smelt release. Only
// services with a registered config path support this (see pkg/workspace);
// NewStack errors for others.
func WithServiceConfig(service, path string) Option {
	return func(c *config) {
		if c.serviceConfigs == nil {
			c.serviceConfigs = map[string]string{}
		}
		c.serviceConfigs[service] = path
	}
}

// WithWorkspaceBinaries builds every service selected by the active Go
// workspace (go.work) from local sibling source and mounts the resulting
// binaries over the published images. Selection follows the use-list: a service
// is built when its module is listed; if libforge is listed, all services are
// rebuilt. Requires an active go.work (see pkg/workspace).
//
// Example:
//
//	s := stack.MustNewStack(t, stack.WithWorkspaceBinaries())
func WithWorkspaceBinaries() Option {
	return func(c *config) {
		c.workspaceBinaries = true
	}
}

// WithWorkspaceBinariesExcept is WithWorkspaceBinaries with the named services
// held back: every other selected service is built from local source, and the
// excluded ones run whatever image the stack resolves for them.
//
// This exists for compatibility testing. In the monorepo the default is that
// everything comes from the working tree, which answers "did my change break
// the system?" -- but not "can the version we are about to ship interoperate
// with the versions already in the field?". Operators upgrade piri and ingot on
// their own schedule, so those run mixed in production. Pinning one service to
// a released image while the rest come from HEAD is exactly that test:
//
//	s := stack.MustNewStack(t,
//	    stack.WithPublishedImages(),
//	    stack.WithWorkspaceBinariesExcept("piri"),
//	    stack.WithPiriImage("ghcr.io/fil-forge/piri:0.2.4"),
//	)
//
// Without the exclusion the workspace binary would be mounted over the pinned
// image, and the test would silently exercise HEAD against HEAD.
func WithWorkspaceBinariesExcept(services ...string) Option {
	return func(c *config) {
		c.workspaceBinaries = true
		if c.workspaceExclude == nil {
			c.workspaceExclude = map[string]bool{}
		}
		for _, s := range services {
			c.workspaceExclude[s] = true
		}
	}
}

// WithGuppyImage sets the guppy container image.
func WithGuppyImage(image string) Option {
	return func(c *config) {
		c.guppyImage = image
	}
}

// WithIndexerImage sets the indexer container image.
func WithIndexerImage(image string) Option {
	return func(c *config) {
		c.indexerImage = image
	}
}

// WithDelegatorImage sets the delegator container image.
func WithDelegatorImage(image string) Option {
	return func(c *config) {
		c.delegatorImage = image
	}
}

// WithUploadImage sets the upload service container image.
func WithUploadImage(image string) Option {
	return func(c *config) {
		c.uploadImage = image
	}
}

// WithHiltImage sets the hilt (tenant management) container image.
func WithHiltImage(image string) Option {
	return func(c *config) {
		c.hiltImage = image
	}
}

// WithSignerImage sets the signing service container image.
func WithSignerImage(image string) Option {
	return func(c *config) {
		c.signerImage = image
	}
}

// WithBlockchainImage sets the blockchain (Anvil) container image.
func WithBlockchainImage(image string) Option {
	return func(c *config) {
		c.blockchainImage = image
	}
}

// WithIPNIImage sets the IPNI container image.
func WithIPNIImage(image string) Option {
	return func(c *config) {
		c.ipniImage = image
	}
}

// WithIngotImage sets the ingot container image.
func WithIngotImage(image string) Option {
	return func(c *config) {
		c.ingotImage = image
	}
}

// WithSwarfImage sets the swarf (UCAN revocation) container image.
func WithSwarfImage(image string) Option {
	return func(c *config) {
		c.swarfImage = image
	}
}

// WithTimeout sets the maximum time to wait for the stack to start.
func WithTimeout(d time.Duration) Option {
	return func(c *config) {
		c.timeout = d
	}
}

// WithKeepOnFailure prevents cleanup when a test fails, useful for debugging.
func WithKeepOnFailure() Option {
	return func(c *config) {
		c.keepOnFailure = true
	}
}

// WithPiriCount configures N identical piri nodes with default storage settings.
//
// Example:
//
//	s := stack.MustNewStack(t, stack.WithPiriCount(3))
func WithPiriCount(n int) Option {
	return func(c *config) {
		c.piriNodes = make([]PiriNodeConfig, n)
	}
}

// WithPiriNodes configures specific piri nodes with individual settings.
//
// Example:
//
//	s := stack.MustNewStack(t, stack.WithPiriNodes(
//	    stack.PiriNodeConfig{Postgres: true, S3: true},
//	    stack.PiriNodeConfig{},
//	))
func WithPiriNodes(nodes ...PiriNodeConfig) Option {
	return func(c *config) {
		c.piriNodes = nodes
	}
}

// WithSnapshot boots the stack from a saved snapshot at a filesystem
// path. Use this for snapshots living outside the smelt module (e.g.
// ones you saved yourself via `smelt snapshot save`, or extras committed
// alongside your tests). For consumers of smelt as a Go dependency,
// prefer WithEmbeddedSnapshot — filesystem paths into the smelt repo
// don't exist in a consumer's checkout.
//
// The snapshot directory must contain the layout produced by `smelt
// snapshot save` (manifest.json, smelt.yml, blockchain/, keys/, proofs/,
// volumes/). Topology comes from the snapshot's embedded smelt.yml —
// pairing with WithPiriCount or WithPiriNodes returns an error from
// NewStack. Running different images against a restored snapshot is
// allowed, and is what testing HEAD against saved state means, so it is
// not an error — but nothing checks it either. NewStack logs the
// snapshot's capture time, the images that wrote the restored state and
// the overrides in force, and leaves the comparison to you.
//
// CI should exercise the cold-boot path. Skip in CI via an env check:
//
//	var opts []stack.Option
//	if os.Getenv("SMELT_TEST_NO_SNAPSHOT") == "" {
//	    opts = append(opts, stack.WithSnapshot("../../snapshots/3-piri-postgres-s3"))
//	}
//	s := stack.MustNewStack(t, opts...)
func WithSnapshot(path string) Option {
	return func(c *config) {
		c.snapshotPath = path
	}
}

// WithEmbeddedSnapshot boots the stack from a snapshot bundled with the
// smelt Go module. This is the recommended path for external consumers
// — the snapshot travels with the Go import, so there are no filesystem
// paths to reason about:
//
//	s := stack.MustNewStack(t,
//	    stack.WithEmbeddedSnapshot("3-piri-postgres-s3"),
//	)
//
// Discover available names at runtime via stack.ListEmbeddedSnapshots().
// Incompatible with WithSnapshot, WithPiriCount, WithPiriNodes. The
// SMELT_TEST_NO_SNAPSHOT env-var skip pattern applies the same as with
// WithSnapshot.
func WithEmbeddedSnapshot(name string) Option {
	return func(c *config) {
		c.embeddedSnapshotName = name
	}
}

// envImageOptions maps each image-override environment variable to its
// option. The variable names match the ones the compose files already
// interpolate (systems/*/compose.yml, and PIRI_IMAGE in pkg/generate), so a
// caller that can set them for `docker compose` can set them for a Go test
// and get the same stack.
var envImageOptions = []struct {
	env string
	opt func(string) Option
}{
	{"PIRI_IMAGE", WithPiriImage},
	{"GUPPY_IMAGE", WithGuppyImage},
	{"INDEXER_IMAGE", WithIndexerImage},
	{"DELEGATOR_IMAGE", WithDelegatorImage},
	{"UPLOAD_IMAGE", WithUploadImage},
	{"HILT_IMAGE", WithHiltImage},
	{"SIGNER_IMAGE", WithSignerImage},
	{"BLOCKCHAIN_IMAGE", WithBlockchainImage},
	{"IPNI_IMAGE", WithIPNIImage},
	{"INGOT_IMAGE", WithIngotImage},
	{"SWARF_IMAGE", WithSwarfImage},
	{"MINIO_IMAGE", WithMinioImage},
	{"PLC_IMAGE", WithPLCImage},
}

// publishedImages is the published reference for each image this repository
// builds, alongside the config field it lands in. It is the Go-side twin of
// .env.published, which says the same thing for `make up` — the Makefile
// shells out to `docker compose` directly, so it cannot read this table.
// Neither copy can drift silently: the compose interpolations for these are
// required (`${X:?...}`), so a missing entry fails at `compose up` naming
// the variable rather than quietly booting :main.
//
// Only what this repository builds. The other images the stack runs (guppy,
// ipni, plc, blockchain, minio) come from elsewhere and keep their `:-`
// compose defaults, so there is nothing for this to fill in.
var publishedImages = []struct {
	ref string
	get func(*config) *string
}{
	{"ghcr.io/fil-forge/piri:main", func(c *config) *string { return &c.piriImage }},
	{"ghcr.io/fil-forge/hilt:main", func(c *config) *string { return &c.hiltImage }},
	{"ghcr.io/fil-forge/ingot:main", func(c *config) *string { return &c.ingotImage }},
	{"ghcr.io/fil-forge/sprue:main", func(c *config) *string { return &c.uploadImage }},
	{"ghcr.io/fil-forge/delegator:main", func(c *config) *string { return &c.delegatorImage }},
	{"ghcr.io/fil-forge/piri-signing-service:main", func(c *config) *string { return &c.signerImage }},
	{"ghcr.io/fil-forge/swarf:main", func(c *config) *string { return &c.swarfImage }},
	{"ghcr.io/fil-forge/indexing-service:main", func(c *config) *string { return &c.indexerImage }},
}

// WithPublishedImages fills in the published :main reference for every image
// this repository builds that the caller has not already set. It is how a
// suite that tests one service against the rest of the network says so: pass
// your own service's image or binary, and take everyone else's from the
// registry.
//
// It exists because the compose files no longer default these images. A
// missing override used to boot ghcr.io/fil-forge/<svc>:main in silence, so
// a test that forgot to pass one passed while exercising published code
// rather than the commit under test. Wanting the published network is now
// something a caller states out loud.
//
// It never overwrites a value already in place, so ordering is what you
// would want either way: put it first and anything appended after it wins.
//
//	opts := []stack.Option{
//	    stack.WithPublishedImages(),
//	    stack.WithServiceBinary("hilt", localHiltBinary(t)),
//	}
//	opts = append(opts, stack.OptionsFromEnv()...) // still wins
func WithPublishedImages() Option {
	return func(c *config) {
		for _, p := range publishedImages {
			if field := p.get(c); *field == "" {
				*field = p.ref
			}
		}
	}
}

// OptionsFromEnv returns the options implied by the process environment: an
// image override per service, plus SMELT_WORKSPACE to build every service in
// the active go.work use-list from local source.
//
// Every test that boots a stack should append these, so that a CI job can
// redirect the whole stack at locally-built images by setting environment
// variables alone. Hand-rolling the mapping per test is what let
// TestStackFromSnapshot run on published :main images inside a job that
// existed to exercise HEAD -- it read neither variable, and nothing said so.
//
// Append these last: an override supplied by the environment should win over
// one a test hard-codes, which is the point of an override.
func OptionsFromEnv() []Option {
	var opts []Option
	for _, m := range envImageOptions {
		if image := os.Getenv(m.env); image != "" {
			opts = append(opts, m.opt(image))
		}
	}
	if os.Getenv("SMELT_WORKSPACE") != "" {
		opts = append(opts, WithWorkspaceBinaries())
	}
	return opts
}

// WithMinioImage overrides the MinIO image. MinIO is not one of our services,
// but upstream withdrew it from every public registry and archived the
// project, so fil-forge/minio builds it from source and the stack points at
// that build.
func WithMinioImage(image string) Option {
	return func(c *config) {
		c.minioImage = image
	}
}

// WithPLCImage overrides the did:web/did:plc directory image.
// systems/plc/compose.yml has interpolated PLC_IMAGE and documented it as the
// override since before this option existed, so until now the documented knob
// worked for docker compose and silently did nothing from a Go test.
func WithPLCImage(image string) Option {
	return func(c *config) {
		c.plcImage = image
	}
}
