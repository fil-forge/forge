//go:build compat

// Package compat answers a question the monorepo does NOT answer for free.
//
// Every commit here is a consistent snapshot, so `e2e` and `itest` boot the
// stack from the working tree and prove HEAD works against HEAD. That is a
// development-time guarantee, and it is the one the repo layout gives you.
//
// Production is not HEAD against HEAD. We run sprue and hilt, but third-party
// storage operators run piri and ingot and upgrade on their own schedule, so
// the field always contains a mix — piri v1.0 talking to an ingot from a
// different release, against whatever sprue we deployed on Tuesday. Rolling out
// two services is never atomic, in either direction.
//
// Before the monorepo, the pinned-version boundary between repos enforced
// expand/contract discipline by accident: changing hilt's client API was
// expensive, so you thought about compatibility. Removing that boundary makes
// atomic cross-service refactors free, which is the point — but it also removes
// the thing that used to make you think. This suite is the deliberate
// replacement.
//
// Two shapes:
//
//	TestPinnedPeer     one service pinned to a RELEASED image, everything else
//	                   from HEAD. "Can what we are about to ship talk to what is
//	                   already deployed?"
//	TestRollingUpgrade boot the whole stack at a released version, then replace
//	                   ONE service with HEAD in place. "Does the upgrade order
//	                   we will actually perform work?" This is the one that
//	                   catches expand/contract violations, because it exercises
//	                   the window where old and new are both live.
//
// Runs behind the `compat` tag, nightly and before a release, never on PRs —
// it needs published images, which a PR's code does not have yet.
//
// # Where the pinned side comes from
//
// The released images are the POLYREPOS' — ghcr.io/fil-forge/<svc> — not this
// repository's. That is not a stopgap standing in for a monorepo registry: the
// polyrepos are what actually ships to the network today, so their images are
// what "already deployed" means. This repository publishes no images at all
// (images.yml builds with push: false) and carries no service-prefixed tags,
// and neither is a prerequisite here.
//
// It follows that a green run says the code in this tree interoperates with
// what the polyrepo released — which is the question — and says nothing about
// artifacts this repository would produce. Only the pinned side is an image;
// the HEAD side is workspace binaries bind-mounted over a base image, so a
// green run is evidence about wire compatibility, not about deployability.
package compat

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/fil-forge/forge/smelt/pkg/clients/guppy"
	"github.com/fil-forge/forge/smelt/pkg/stack"
	"github.com/fil-forge/forge/smelt/pkg/workspace"
)

// imageRepo is where the services' released images live. Kept as one constant
// so a registry move is a single edit.
const imageRepo = "ghcr.io/fil-forge"

// imageFor builds the image reference for a released version.
//
// The leading `v` is stripped, and that is load-bearing rather than tidiness:
// the polyrepos tag git with `v0.2.4` and publish the image as `0.2.4`.
// Measured against the registry — ghcr.io/fil-forge/piri:0.2.4 answers 200 and
// :v0.2.4 answers 404 — so passing a git tag through unchanged pulls nothing
// and the failure surfaces as an unrelated container timeout.
func imageFor(service, version string) string {
	return fmt.Sprintf("%s/%s:%s", imageRepo, service, strings.TrimPrefix(version, "v"))
}

func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	if err := stack.CleanupLeaked(ctx); err != nil {
		log.Printf("compat: pre-test sweep warning: %v", err)
	}
	cancel()
	os.Exit(m.Run())
}

// pinnedVersions reads the versions to test against from the environment, as
// COMPAT_<SERVICE>_VERSIONS, comma separated. The workflow fills these from the
// version-shaped tags that actually exist in the registry; a developer can set
// one by hand.
//
//	COMPAT_PIRI_VERSIONS=0.2.4 go test -tags compat ./tests/compat
//
// Skips rather than fails when unset: a service with no published releases has
// nothing to test against, and that should not be a permanent red.
func pinnedVersions(t *testing.T, service string) []string {
	t.Helper()
	key := "COMPAT_" + strings.ToUpper(service) + "_VERSIONS"
	raw := os.Getenv(key)
	if strings.TrimSpace(raw) == "" {
		t.Skipf("%s unset; no published release window for %s", key, service)
	}
	var out []string
	for _, v := range strings.Split(raw, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// pinFor maps a smelt service name to the option that pins its image. The
// names are workspace.Detect()'s, which are the names the stack itself uses,
// so TestRollingUpgrade can walk that list and pin every entry.
//
// A service in the workspace with no entry here is a hard failure rather than
// a default, because the default is the bug this table exists to stop: a
// service nobody pinned is built from HEAD, and a fleet with one HEAD service
// in it that nobody meant to put there is not a baseline.
var pinFor = map[string]func(string) stack.Option{
	"piri":            stack.WithPiriImage,
	"ingot":           stack.WithIngotImage,
	"upload":          stack.WithUploadImage,
	"hilt":            stack.WithHiltImage,
	"indexer":         stack.WithIndexerImage,
	"delegator":       stack.WithDelegatorImage,
	"signing-service": stack.WithSignerImage,
	"swarf":           stack.WithSwarfImage,
	"guppy":           stack.WithGuppyImage,
}

// envSuffix is a service's name as it appears in COMPAT_BASELINE_<SUFFIX>.
func envSuffix(service string) string {
	return strings.ToUpper(strings.ReplaceAll(service, "-", "_"))
}

// TestPinnedPeer boots the stack with one service at a released image and every
// other in-repo service built from this commit.
func TestPinnedPeer(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("skipping on darwin (docker-in-docker flakiness)")
	}

	for _, service := range []string{"piri", "ingot"} {
		t.Run(service, func(t *testing.T) {
			for _, version := range pinnedVersions(t, service) {
				t.Run(version, func(t *testing.T) {
					image := imageFor(service, version)
					pin, ok := pinFor[service]
					if !ok {
						t.Fatalf("no pin option for %s", service)
					}

					// WithPublishedImages is not garnish. Every service
					// this repository builds is a REQUIRED compose
					// interpolation -- ${HILT_IMAGE:?...} and its
					// siblings -- and config.buildEnv() omits a variable
					// it has no value for, so compose refuses to start,
					// naming the variable. Workspace binaries do not
					// cover for that: they are bind mounts over an image,
					// not an image.
					//
					// The exclusion is load-bearing too. Without it the
					// workspace binary is mounted over the pinned image
					// and this quietly becomes a HEAD-vs-HEAD run that
					// always passes.
					s := stack.MustNewStack(t,
						stack.WithPublishedImages(),
						stack.WithPiriNodes(stack.PiriNodeConfig{Postgres: true}),
						stack.WithWorkspaceBinariesExcept(service),
						pin(image),
					)
					t.Logf("compat: %s pinned to %s, all other services from HEAD", service, image)

					assertUploadRetrieve(t, s)
				})
			}
		})
	}
}

// TestRollingUpgrade boots every in-repo service at a released version, then
// replaces one with HEAD while the rest stay old -- the state a real deployment
// passes through, since services do not upgrade simultaneously.
//
// Compared to TestPinnedPeer this inverts which side is new: there, one old
// service meets a new fleet; here, one new service meets an old fleet. Both
// windows occur during a rollout, and a change can break only one of them
// (adding a required field breaks old-reader/new-writer; removing one breaks
// new-reader/old-writer).
//
// THE FLEET IS WHATEVER THE WORKSPACE WOULD BUILD, not a list written here.
// That is the whole correctness condition: any service left off such a list is
// built from HEAD, so "the rest stay old" silently stops being true for it. A
// hand-written four -- piri, ingot, sprue, hilt -- was wrong by four against
// this go.work, and one of the four it missed was the indexer, which sits on
// the path this test asserts over.
//
// So it needs a released image for every one of them, and skips naming the
// ones it lacks. sprue and hilt publish no version-tagged images today (only
// :main and :sha-*), and neither do the indexer, delegator, signing service or
// swarf, so the skip is the expected outcome until they cut releases. It is a
// skip rather than a substitution on purpose: :main is a moving tag, and
// pinning a compatibility baseline to something that moves is how a red
// becomes ambiguous.
func TestRollingUpgrade(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("skipping on darwin (docker-in-docker flakiness)")
	}

	_, fleet, err := workspace.Detect()
	if err != nil {
		t.Skipf("no active go.work, so there is no fleet to hold old: %v", err)
	}

	// Per-service semver means there is no single baseline VERSION -- there is
	// a baseline SET, one release per service. COMPAT_BASELINE_<SERVICE>
	// carries each; the workflow fills them from the registry.
	baseline := map[string]string{}
	var missing []string
	for _, svc := range fleet {
		if _, ok := pinFor[svc]; !ok {
			t.Fatalf("workspace service %q has no entry in pinFor, so this test "+
				"cannot pin it and would boot it from HEAD inside a fleet that is "+
				"supposed to be old. Add it.", svc)
		}
		v := strings.TrimSpace(os.Getenv("COMPAT_BASELINE_" + envSuffix(svc)))
		if v == "" {
			missing = append(missing, svc)
			continue
		}
		baseline[svc] = v
	}
	if len(missing) > 0 {
		t.Skipf("no published release to upgrade from for %s "+
			"(set COMPAT_BASELINE_<SERVICE> for each)", strings.Join(missing, ", "))
	}

	for _, upgraded := range []string{"piri", "ingot"} {
		t.Run("upgrade_"+upgraded, func(t *testing.T) {
			opts := []stack.Option{
				stack.WithPublishedImages(),
				stack.WithPiriNodes(stack.PiriNodeConfig{Postgres: true}),
			}
			// Everything from the released baseline set...
			for _, svc := range fleet {
				opts = append(opts, pinFor[svc](imageFor(svc, baseline[svc])))
			}
			// ...except the one service under upgrade, which comes from HEAD.
			// Excluding every other workspace service is what keeps the rest
			// genuinely old; the pin above is undone by a bind mount otherwise.
			opts = append(opts, stack.WithWorkspaceBinariesExcept(otherThan(fleet, upgraded)...))

			s := stack.MustNewStack(t, opts...)
			t.Logf("compat: fleet at baseline set %v, %s upgraded to HEAD", baseline, upgraded)

			assertUploadRetrieve(t, s)
		})
	}
}

// otherThan returns every service in the fleet except the named one, for use
// as the exclusion list -- i.e. "build only `service` from HEAD".
func otherThan(fleet []string, service string) []string {
	var out []string
	for _, s := range fleet {
		if s != service {
			out = append(out, s)
		}
	}
	return out
}

// assertUploadRetrieve drives the full network path — guppy -> sprue -> piri ->
// indexer and back — which is the interaction that actually has to stay
// wire-compatible across versions. Mirrors the e2e smoke assertion.
func assertUploadRetrieve(t *testing.T, s *stack.Stack) {
	t.Helper()
	ctx := t.Context()

	waitHTTPOK(t, s.IngotEndpoint()+"/health", 3*time.Minute)

	gup, err := guppy.NewContainerClient(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := gup.Login(ctx, "compat@example.com"); err != nil {
		t.Fatalf("login: %v", err)
	}
	spaceDID, err := gup.GenerateSpace(ctx)
	if err != nil {
		t.Fatalf("generate space: %v", err)
	}
	dataPath, err := gup.GenerateTestData(ctx, "10MB")
	if err != nil {
		t.Fatalf("generate test data: %v", err)
	}
	if err := gup.AddSource(ctx, spaceDID, dataPath); err != nil {
		t.Fatalf("add source: %v", err)
	}
	cids, err := gup.Upload(ctx, spaceDID, guppy.WithReplicas(1))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if len(cids) == 0 {
		t.Fatal("expected at least one CID from upload")
	}
	dst := fmt.Sprintf("/tmp/compat-download-%d", time.Now().UnixNano())
	if err := gup.Retrieve(ctx, spaceDID, cids[len(cids)-1], dst); err != nil {
		t.Fatalf("retrieve: %v", err)
	}
}
