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
// Every name in this suite — which services exist, what smelt calls them, what
// the registry calls them, how each is pinned — comes from the one table below
// (services). The suite used to spell the compose names in one list and the
// image names in another, and an edit narrowing either would have silently
// turned a pinned run into HEAD-vs-HEAD; TestServiceTable pins the table to
// what pkg/workspace can build.
//
// Runs behind the `compat` tag: nightly, before a release, and on demand with
// literal image tags (see .github/workflows/compat.yml). Never on PRs — it
// needs published images, which a PR's code does not have yet.
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

// service is one row of the table every name in this suite derives from.
//
// An in-repo service has three names, and they do not all agree: smelt's
// compose calls sprue "upload" (the service name — the key that
// WithWorkspaceBinariesExcept, container lookup and pkg/workspace all use),
// the registry calls it "sprue" (the image under imageRepo, the COMPAT_* env
// suffix, the workflow's vocabulary), and its binary lives at /usr/bin/sprue.
type service struct {
	// compose is the smelt compose service name. piri fans out to piri-N
	// nodes; see stack.PiriServiceNames for the live names.
	compose string
	// image is the image name under imageRepo and the COMPAT_<IMAGE>_*
	// environment suffix (upper-cased).
	image string
	// pin is the stack option that pins the service to an image reference.
	pin func(image string) stack.Option
	// operatorRun marks the services third parties run on their own
	// schedule — the ones that lag in the field, and so the ones this suite
	// pins (TestPinnedPeer) and upgrades one at a time (TestRollingUpgrade).
	// sprue and hilt we deploy ourselves and can upgrade together, so a skew
	// between them is our own scheduling problem rather than a compatibility
	// contract.
	operatorRun bool
}

// services is THE table: every in-repo service the compat suite knows about.
var services = []service{
	{compose: "piri", image: "piri", pin: stack.WithPiriImage, operatorRun: true},
	{compose: "ingot", image: "ingot", pin: stack.WithIngotImage, operatorRun: true},
	{compose: "upload", image: "sprue", pin: stack.WithUploadImage},
	{compose: "hilt", image: "hilt", pin: stack.WithHiltImage},
}

// binPath is where the service's binary lives inside its image, and therefore
// where pkg/workspace bind-mounts a HEAD build. It is read from the workspace
// table rather than repeated here, so the suite cannot drift from the code
// that does the mounting.
func (s service) binPath() (string, error) {
	p, ok := workspace.BinPath(s.compose)
	if !ok {
		return "", fmt.Errorf("compose service %q (image %q) is unknown to pkg/workspace; known: %s", s.compose, s.image, strings.Join(workspace.ServiceNames(), ", "))
	}
	return p, nil
}

// ref is the full image reference for the service at an image tag. The tag is
// an opaque image tag — a release (v1.2.0) or a per-commit sha-<short> that
// publish-ghcr.yml pushes for every commit on main.
func (s service) ref(tag string) string {
	return fmt.Sprintf("%s/%s:%s", imageRepo, s.image, tag)
}

// operatorRun returns the rows that get pinned / upgraded one at a time.
func operatorRun() []service {
	var out []service
	for _, s := range services {
		if s.operatorRun {
			out = append(out, s)
		}
	}
	return out
}

// otherThan returns the compose name of every in-repo service except svc, for
// use as the WithWorkspaceBinariesExcept list — i.e. "build only svc from
// HEAD". Derived from the table, so it can only fall out of step with the
// baseline set by editing the table, which TestServiceTable checks.
func otherThan(svc service) []string {
	var out []string
	for _, s := range services {
		if s.compose != svc.compose {
			out = append(out, s.compose)
		}
	}
	return out
}

// pinnedVersions reads the image tags to test against from the environment,
// as COMPAT_<IMAGE>_VERSIONS, comma separated. The workflow sets these to the
// currently-supported window, or to the literal tags of a dispatch; a
// developer can set one by hand:
//
//	COMPAT_PIRI_VERSIONS=sha-96a672e,v1.0.0 go test -tags compat ./tests/compat
//
// Skips when unset: there is no supported window to test against until there
// are releases, and a repo that has not cut one yet should not have a
// permanently red nightly. The skip is visible, not silent — the workflow
// fails a run in which nothing at all was exercised.
func pinnedVersions(t *testing.T, svc service) []string {
	t.Helper()
	key := "COMPAT_" + strings.ToUpper(svc.image) + "_VERSIONS"
	raw := os.Getenv(key)
	if strings.TrimSpace(raw) == "" {
		t.Skipf("%s unset; no supported release window declared for %s", key, svc.image)
	}
	var out []string
	for _, v := range strings.Split(raw, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// baselineSet reads COMPAT_BASELINE_<IMAGE> for every in-repo service and
// returns image name -> tag. Per-service semver means there is no single
// baseline VERSION — there is a baseline SET, one release per service; the
// workflow fills them from the latest tag of each service, or verbatim from a
// dispatch. Skips when any is unset.
func baselineSet(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, svc := range services {
		key := "COMPAT_BASELINE_" + strings.ToUpper(svc.image)
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" {
			t.Skipf("%s unset; no released baseline set to upgrade from", key)
		}
		out[svc.image] = v
	}
	return out
}

// baseOptions is the topology every compat stack shares.
func baseOptions(t *testing.T) []stack.Option {
	t.Helper()
	return []stack.Option{stack.WithPiriNodes(stack.PiriNodeConfig{Postgres: true})}
}

// TestPinnedPeer boots the stack with one service at a released image and every
// other in-repo service built from this commit.
func TestPinnedPeer(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("skipping on darwin (docker-in-docker flakiness)")
	}

	for _, svc := range operatorRun() {
		t.Run(svc.image, func(t *testing.T) {
			for _, version := range pinnedVersions(t, svc) {
				t.Run(version, func(t *testing.T) {
					image := svc.ref(version)

					// The exclusion is load-bearing: without it the workspace
					// binary would be mounted over the pinned image and this
					// would quietly become a HEAD-vs-HEAD run that always
					// passes.
					opts := append(baseOptions(t),
						stack.WithWorkspaceBinariesExcept(svc.compose),
						svc.pin(image),
					)
					s := stack.MustNewStack(t, opts...)
					t.Logf("compat: %s pinned to %s, all other services from HEAD", svc.image, image)

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

	baseline := baselineSet(t)

	for _, upgraded := range operatorRun() {
		t.Run("upgrade_"+upgraded.image, func(t *testing.T) {
			// Everything from the released baseline set — including the
			// service under upgrade, whose baseline image is what the HEAD
			// binary is mounted over, so no floating :main is pulled...
			opts := baseOptions(t)
			for _, svc := range services {
				opts = append(opts, svc.pin(svc.ref(baseline[svc.image])))
			}
			// ...except that the one service under upgrade comes from HEAD.
			// Building only that service keeps the rest genuinely old.
			opts = append(opts, stack.WithWorkspaceBinariesExcept(otherThan(upgraded)...))

			s := stack.MustNewStack(t, opts...)
			t.Logf("compat: fleet at baseline set %v, %s upgraded to HEAD", baseline, upgraded.image)

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
