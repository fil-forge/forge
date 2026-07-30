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

// pinOption maps a service name to the stack option that pins its image.
// Only the operator-facing services are covered: piri and ingot are the ones
// third parties run and therefore the ones that can lag. sprue and hilt we
// deploy ourselves and can upgrade together, so a version skew between them is
// our own scheduling problem rather than a compatibility contract.
func pinOption(service, image string) (stack.Option, bool) {
	switch service {
	case "piri":
		return stack.WithPiriImage(image), true
	case "ingot":
		return stack.WithIngotImage(image), true
	default:
		return nil, false
	}
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
					image := fmt.Sprintf("%s/%s:%s", imageRepo, service, version)
					pin, ok := pinOption(service, image)
					if !ok {
						t.Fatalf("no pin option for %s", service)
					}

					// The exclusion is load-bearing: without it the workspace
					// binary would be mounted over the pinned image and this
					// would quietly become a HEAD-vs-HEAD run that always
					// passes.
					s := stack.MustNewStack(t,
						stack.WithPiriNodes(stack.PiriNodeConfig{Postgres: true}),
						stack.WithWorkspaceBinariesExcept(service),
						pin,
					)
					t.Logf("compat: %s pinned to %s, all other services from HEAD", service, image)

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
			opts := []stack.Option{
				stack.WithPiriNodes(stack.PiriNodeConfig{Postgres: true}),
				// Everything from the released baseline set...
				stack.WithPiriImage(fmt.Sprintf("%s/piri:%s", imageRepo, baseline["piri"])),
				stack.WithIngotImage(fmt.Sprintf("%s/ingot:%s", imageRepo, baseline["ingot"])),
				stack.WithUploadImage(fmt.Sprintf("%s/sprue:%s", imageRepo, baseline["sprue"])),
				stack.WithHiltImage(fmt.Sprintf("%s/hilt:%s", imageRepo, baseline["hilt"])),
			}
			// ...except the one service under upgrade, which comes from HEAD.
			// Building only that service keeps the rest genuinely old.
			opts = append(opts, stack.WithWorkspaceBinariesExcept(otherThan(upgraded)...))

			s := stack.MustNewStack(t, opts...)
			t.Logf("compat: fleet at baseline set %v, %s upgraded to HEAD", baseline, upgraded)

			assertUploadRetrieve(t, s)
		})
	}
}

// otherThan returns every in-repo service except the named one, for use as the
// exclusion list — i.e. "build only `service` from HEAD".
func otherThan(service string) []string {
	all := []string{"piri", "ingot", "upload", "hilt"}
	var out []string
	for _, s := range all {
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
