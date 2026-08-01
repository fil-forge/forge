//go:build compat

// Package compat answers a question the monorepo does NOT answer for free.
//
// Every commit here is a consistent snapshot, so the stack job in ci.yml boots
// all five services from the working tree and proves HEAD works against HEAD.
// That is a development-time guarantee, and it is the one the repo layout
// gives you.
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
)

// imageRepo is where the monorepo publishes service images. Kept as one
// constant so a registry move is a single edit.
const imageRepo = "ghcr.io/fil-forge/forge"

func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	if err := stack.CleanupLeaked(ctx); err != nil {
		log.Printf("compat: pre-test sweep warning: %v", err)
	}
	cancel()
	os.Exit(m.Run())
}

// pinnedVersions reads the versions to test against from the environment, as
// COMPAT_<SERVICE>_VERSIONS, comma separated. The release workflow sets these
// to the currently-supported window; a developer can set one by hand.
//
//	COMPAT_PIRI_VERSIONS=v1.0.0,v1.1.0 go test -tags compat ./tests/compat
//
// Returns nil when unset, which skips rather than fails: there is no supported
// window to test against until there are releases, and a repo that has not cut
// one yet should not have a permanently red nightly.
func pinnedVersions(t *testing.T, service string) []string {
	t.Helper()
	key := "COMPAT_" + strings.ToUpper(service) + "_VERSIONS"
	raw := os.Getenv(key)
	if strings.TrimSpace(raw) == "" {
		t.Skipf("%s unset; no supported release window declared for %s", key, service)
	}
	var out []string
	for _, v := range strings.Split(raw, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// serviceImageOption maps an in-repo service name to the stack option that
// sets its container image. Keys follow the baseline/env naming: piri, ingot,
// sprue, hilt.
func serviceImageOption(service, image string) (stack.Option, bool) {
	switch service {
	case "piri":
		return stack.WithPiriImage(image), true
	case "ingot":
		return stack.WithIngotImage(image), true
	case "sprue":
		return stack.WithUploadImage(image), true
	case "hilt":
		return stack.WithHiltImage(image), true
	default:
		return nil, false
	}
}

// headImageEnv names the environment variable carrying each service's
// THIS-COMMIT image reference. The compat workflow builds those images in-job
// (forge-ci/<svc>:<sha>); a local run gets them from `make images`
// (forge/<svc>:local).
var headImageEnv = map[string]string{
	"piri":  "PIRI_IMAGE",
	"ingot": "INGOT_IMAGE",
	"sprue": "UPLOAD_IMAGE",
	"hilt":  "HILT_IMAGE",
}

// headImages resolves the image references for this commit's services from
// the environment, skipping when any is unset: containers are the only way
// code enters a stack, so HEAD images are the suite's precondition — it never
// compiles or mounts anything itself.
func headImages(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for svc, key := range headImageEnv {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" {
			t.Skipf("%s unset; HEAD images are required (CI builds them in-job; locally run `make images` and export %s=forge/%s:local)", key, key, svc)
		}
		out[svc] = v
	}
	return out
}

// TestPinnedPeer boots the stack with one service at a released image and every
// other in-repo service built from this commit.
func TestPinnedPeer(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("skipping on darwin (docker-in-docker flakiness)")
	}

	// Only the operator-facing services get pinned: piri and ingot are the
	// ones third parties run and therefore the ones that can lag. sprue and
	// hilt we deploy ourselves and can upgrade together, so a version skew
	// between them is our own scheduling problem, not a compat contract.
	for _, service := range []string{"piri", "ingot"} {
		t.Run(service, func(t *testing.T) {
			for _, version := range pinnedVersions(t, service) {
				t.Run(version, func(t *testing.T) {
					head := headImages(t)
					image := fmt.Sprintf("%s/%s:%s", imageRepo, service, version)

					// Every service runs a container, nothing is mounted:
					// HEAD images for the fleet, the released image for the
					// one pinned service. Setting all four explicitly means
					// no reliance on env-vs-option precedence — what each
					// container runs is spelled out here.
					opts := []stack.Option{stack.WithPiriNodes(stack.PiriNodeConfig{Postgres: true})}
					for svc, img := range head {
						if svc == service {
							img = image
						}
						o, ok := serviceImageOption(svc, img)
						if !ok {
							t.Fatalf("no image option for %s", svc)
						}
						opts = append(opts, o)
					}

					s := stack.MustNewStack(t, opts...)
					t.Logf("compat: %s pinned to %s, all other services at HEAD images", service, image)

					assertUploadRetrieve(t, s)
				})
			}
		})
	}
}

// TestRollingUpgrade boots every in-repo service at a released version, then
// replaces one with HEAD while the rest stay old — the state a real deployment
// passes through, since services do not upgrade simultaneously.
//
// Compared to TestPinnedPeer this inverts which side is new: there, one old
// service meets a new fleet; here, one new service meets an old fleet. Both
// windows occur during a rollout, and a change can break only one of them
// (adding a required field breaks old-reader/new-writer; removing one breaks
// new-reader/old-writer).
func TestRollingUpgrade(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("skipping on darwin (docker-in-docker flakiness)")
	}

	// Per-service semver means there is no single baseline VERSION — there is a
	// baseline SET, one release per service. COMPAT_BASELINE_<SERVICE> carries
	// each; the workflow fills them from the latest tag of each service.
	baseline := map[string]string{}
	for _, svc := range []string{"piri", "ingot", "sprue", "hilt"} {
		v := strings.TrimSpace(os.Getenv("COMPAT_BASELINE_" + strings.ToUpper(svc)))
		if v == "" {
			t.Skipf("COMPAT_BASELINE_%s unset; no released baseline set to upgrade from", strings.ToUpper(svc))
		}
		baseline[svc] = v
	}

	for _, upgraded := range []string{"piri", "ingot"} {
		t.Run("upgrade_"+upgraded, func(t *testing.T) {
			head := headImages(t)

			// Everything runs the released baseline image except the one
			// service under upgrade, which runs THIS commit's image. All
			// containers, nothing mounted — the upgraded service's packaging
			// is the new packaging too, exactly what a real upgrade ships.
			opts := []stack.Option{stack.WithPiriNodes(stack.PiriNodeConfig{Postgres: true})}
			for _, svc := range []string{"piri", "ingot", "sprue", "hilt"} {
				img := fmt.Sprintf("%s/%s:%s", imageRepo, svc, baseline[svc])
				if svc == upgraded {
					img = head[svc]
				}
				o, ok := serviceImageOption(svc, img)
				if !ok {
					t.Fatalf("no image option for %s", svc)
				}
				opts = append(opts, o)
			}

			s := stack.MustNewStack(t, opts...)
			t.Logf("compat: fleet at baseline set %v, %s upgraded to HEAD image %s", baseline, upgraded, head[upgraded])

			assertUploadRetrieve(t, s)
		})
	}
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
