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
// One shape:
//
//	TestRollingUpgrade boot the whole stack at a released version, then replace
//	                   ONE service with HEAD in place. "Does the upgrade order
//	                   we will actually perform work?" It catches
//	                   expand/contract violations, because it exercises the
//	                   window where old and new are both live.
//
// Runs behind the `compat` tag, on a release pull request and nowhere else on
// its own -- that is the one place its answer has a decision attached, since
// red there means do not merge this release. `compat-refresh.yml` re-runs it
// daily on every open release pull request, because the pinned side is read
// live from the registry and so the answer goes stale without the code moving.
// GitHub allows that for 30 days after the original run and no longer, so a
// release pull request open past a month keeps an answer nothing refreshes.
//
// # Where the pinned side comes from
//
// The images are the POLYREPOS' — ghcr.io/fil-forge/<svc> — not this
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
//
// # Two kinds of baseline
//
// COMPAT_BASELINE_<PKG> carries either a version (`0.2.4`) or a digest
// (`sha256:…`), and the difference is the difference between a cut release and
// upstream's current main. Five of the eight services have cut no release, so
// their baseline is `:main` resolved to a digest by
// .github/scripts/compat-baselines.sh, whose header says what that is worth.
//
// The short version: those five are ones we deploy ourselves, so skew between
// them is a scheduling problem rather than a compatibility contract, and the
// skew this suite is about comes from piri and ingot — which third parties run
// on their own schedule, and which do have releases to pin to. A run where
// NONE of the baselines is a release is a run comparing current against
// current, and this file fails rather than reporting that green.
package compat

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime"
	"slices"
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

// imageFor builds the image reference for a released version of a service.
//
// THE SERVICE NAME IS NOT THE PACKAGE NAME for three of the nine, and passing it
// through is a silent 403: measured at the token endpoint, fil-forge/upload,
// fil-forge/indexer and fil-forge/signing-service all answer 403 while
// fil-forge/sprue, fil-forge/indexing-service and fil-forge/piri-signing-service
// answer 200. workspace.ModuleDir is the mapping, and it is the same one that
// decides what gets built, so the two cannot disagree.
//
// A DIGEST IS JOINED WITH `@`, A VERSION WITH `:`, and `sha256:` is what tells
// them apart. Both arrive through the same variable because a service with no
// cut release is pinned to `:main` resolved to a digest; joining a digest with
// `:` asks for ghcr.io/fil-forge/<pkg>:sha256:… , which is not a tag anything
// published.
//
// The leading `v` is stripped, and that is load-bearing rather than tidiness:
// the polyrepos tag git with `v0.2.4` and publish the image as `0.2.4`.
// Measured against the registry — ghcr.io/fil-forge/piri:0.2.4 answers 200 and
// :v0.2.4 answers 404 — so passing a git tag through unchanged pulls nothing
// and the failure surfaces as an unrelated container timeout.
func imageFor(t *testing.T, service, version string) string {
	t.Helper()
	pkg, ok := workspace.ModuleDir(service)
	if !ok {
		t.Fatalf("no module directory known for service %q, so its registry "+
			"package name cannot be derived", service)
	}
	if isDigest(version) {
		return fmt.Sprintf("%s/%s@%s", imageRepo, pkg, version)
	}
	return fmt.Sprintf("%s/%s:%s", imageRepo, pkg, strings.TrimPrefix(version, "v"))
}

// isDigest reports whether a baseline value is a digest rather than a tag.
// imageFor uses it to pick the separator, and nothing else: it is not the
// discriminator for "is this a release", which is isRelease below.
func isDigest(v string) bool { return strings.HasPrefix(v, "sha256:") }

// isRelease matches a cut release, and it matches POSITIVELY rather than as
// the complement of isDigest. That distinction is the whole guard: "not a
// digest" counts `main`, `main-dev`, `latest` and `sha-abc1234` as releases,
// and `main` is a value this very file assigns two ways -- the unset fallback
// below, and TestImageFor's floating-tag case. With the complement, one
// hand-set COMPAT_BASELINE_PIRI=main puts every service on :main and still
// satisfies the all-floating check, which is exactly the vacuous run that
// check exists to refuse.
//
// The pattern is versions_from's in compat-baselines.sh, plus the optional
// leading `v` imageFor already strips.
var isRelease = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+$`).MatchString

// TestImageFor locks the one thing imageFor can get wrong that nothing else
// would catch until a container timed out forty minutes later: a digest joined
// with `:`. It runs before the stack, costs nothing, and needs no Docker.
func TestImageFor(t *testing.T) {
	const dig = "sha256:a71a8bdcfd45e227019b9c5c8be344481f263a8070c00431cce520c31266e2d6"
	for _, c := range []struct {
		name, service, version, want string
	}{
		// The service name is not the package name for three of them, which is
		// the other way this used to go wrong -- so the cases that exercise
		// the separator use the services where both facts are live at once.
		{"release", "piri", "0.2.4", "ghcr.io/fil-forge/piri:0.2.4"},
		{"release with a v", "piri", "v0.2.4", "ghcr.io/fil-forge/piri:0.2.4"},
		{"renamed service", "upload", "1.2.3", "ghcr.io/fil-forge/sprue:1.2.3"},
		{"digest", "hilt", dig, "ghcr.io/fil-forge/hilt@" + dig},
		{"digest, renamed service", "signing-service", dig,
			"ghcr.io/fil-forge/piri-signing-service@" + dig},
		{"floating tag", "swarf", "main", "ghcr.io/fil-forge/swarf:main"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := imageFor(t, c.service, c.version); got != c.want {
				t.Errorf("imageFor(%q, %q) = %q, want %q",
					c.service, c.version, got, c.want)
			}
		})
	}
}

func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	if err := stack.CleanupLeaked(ctx); err != nil {
		log.Printf("compat: pre-test sweep warning: %v", err)
	}
	cancel()
	os.Exit(m.Run())
}

// A VERSION-SHAPED TAG HERE IS A CUT RELEASE: upstream publishes
// <svc>:X.Y.Z only from the git tag vX.Y.Z, never from an ordinary push to
// main, which produces :main and :sha-* instead. That is what makes a version
// baseline worth more than the :main fallback: it does not move, and it is
// genuinely older than HEAD.
//
// The exact mechanism is NOT described here, on purpose. Three revisions of
// this comment tried and each was wrong in a different way -- which commit
// carries the tag, which workflow pushes the image, whether a tag can be
// republished -- and none of those details changes what a reader of this file
// does. The chain differs per service and lives in each polyrepo's
// .github/workflows; read it there if you need it.
//
// A GIT TAG IS NOT A RELEASE HERE. hilt and swarf both carry v0.0.0 in git and
// publish no image under it -- hilt's is on the commit that ADDED version.json
// and swarf's is on its root commit, six weeks before it had one. The registry
// is the only thing that answers the question this file has, which is why the
// workflow reads the registry and not git.
//
// What ingot's 0.0.0 says is how OLD the pinned peer is: fil-forge/ingot
// carries exactly one version-tagged image, so it is that service's first and
// only release.

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

// envSuffix is a service's PACKAGE name as it appears in
// COMPAT_BASELINE_<SUFFIX>, not its service name. The workflow fills these from
// the registry, which knows only package names, so keying on the service name
// made it feed COMPAT_BASELINE_SPRUE to a test reading COMPAT_BASELINE_UPLOAD.
func envSuffix(t *testing.T, service string) string {
	t.Helper()
	pkg, ok := workspace.ModuleDir(service)
	if !ok {
		t.Fatalf("no module directory known for service %q", service)
	}
	return strings.ToUpper(strings.ReplaceAll(pkg, "-", "_"))
}

// TestRollingUpgrade boots every in-repo service at a released version, then
// replaces one with HEAD while the rest stay old -- the state a real deployment
// passes through, since services do not upgrade simultaneously.
//
// One new service meeting an old fleet: the direction the gate's question is
// about.
//
// THE FLEET IS WHATEVER THE WORKSPACE WOULD BUILD, not a list written here.
// That is the whole correctness condition: any service left off such a list is
// built from HEAD, so "the rest stay old" silently stops being true for it. A
// hand-written four -- piri, ingot, sprue, hilt -- was wrong by four against
// this go.work, and one of the four it missed was the indexer, which sits on
// the path this test asserts over.
//
// So it needs an image for every one of them. Five of the eight publish no
// version-tagged image today -- sprue, hilt, the signing service, delegator
// and swarf. What they do publish is :main and :sha-<sha>, and, for hilt,
// sprue and swarf, :main-dev and :sha-<sha>-dev alongside them. The other three do -- piri 0.2.4, ingot 0.0.0
// and the INDEXER at 1.13.4, which is worth naming because an earlier revision
// of this comment counted it among the missing.
//
// IT USED TO SKIP OVER THE FIVE, AND THAT MADE THE WHOLE GATE VACUOUS. The
// workflow ran the job when ANY service could be pinned; this needed EVERY
// service pinned; so the job ran, this skipped, `go test` exited 0, and the
// check went green in 0.126s having booted nothing. Two conditions differing
// by one quantifier with a green check in the gap.
//
// Now the five take `:main`, and the objection the skip was defending against
// is answered where it arises rather than by not running: :main moves, so the
// workflow resolves it to a DIGEST and that digest is in the run's log and
// output. The run still faces whatever :main is at the moment it asks, and a
// red is still reproducible afterwards.
//
// What the fallback does NOT buy is skew for those five -- :main is roughly
// this tree, so they contribute almost none. That is the right outcome rather
// than a compromise: we deploy those five ourselves and can upgrade them
// together. The skew this test is about comes from piri and ingot, the two it
// upgrades, and those two have releases.
func TestRollingUpgrade(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("skipping on darwin (docker-in-docker flakiness)")
	}

	// FATAL, NOT A SKIP. Every other vacuous path in this file was hardened
	// into a failure by the change that removed the baseline skip, and this
	// one was left: a workspace that cannot be detected gave `--- SKIP` and
	// rc=0, which is the 0.126s green in a different costume. Nothing in CI
	// reaches it -- compat.yml runs with GOWORK on and asserts so before the
	// test, and ci.yml's `vet tagged suites` only compiles this package -- so
	// the only way here is a local run without a workspace, where the suite
	// cannot answer its question anyway.
	_, fleet, err := workspace.Detect()
	if err != nil {
		t.Fatalf("no active go.work, so there is no fleet to hold old and "+
			"nothing to upgrade from HEAD: %v", err)
	}

	// Per-service semver means there is no single baseline VERSION -- there is
	// a baseline SET, one entry per service. COMPAT_BASELINE_<PACKAGE> carries
	// each; the workflow fills them from the registry.
	baseline := map[string]string{}
	var releases, floating []string
	for _, svc := range fleet {
		if _, ok := pinFor[svc]; !ok {
			t.Fatalf("workspace service %q has no entry in pinFor, so this test "+
				"cannot pin it and would boot it from HEAD inside a fleet that is "+
				"supposed to be old. Add it.", svc)
		}
		v := strings.TrimSpace(os.Getenv("COMPAT_BASELINE_" + envSuffix(t, svc)))
		if v == "" {
			// Unset means nobody resolved one, which in practice means a local
			// run: the workflow sets every entry. Take the bare tag rather
			// than skipping, so a local run boots something.
			v = "main"
		}
		if isRelease(v) {
			releases = append(releases, svc)
		} else {
			floating = append(floating, svc)
		}
		baseline[svc] = v
	}
	// THE ONE THING THAT WOULD MAKE THIS VACUOUS AGAIN. With no release in the
	// set the whole fleet is upstream's main, which is roughly this tree, so
	// the run compares current against current and a green means nothing. That
	// is a failure rather than a skip, because a skip is what put this file
	// here.
	if len(releases) == 0 {
		t.Fatalf("every baseline is a floating :main (%v), so the old half of "+
			"the fleet is the same code as HEAD and a green here would assert "+
			"nothing. Resolve them with:\n"+
			"  eval \"$(.github/scripts/compat-baselines.sh | "+
			"sed -n 's/^base_/export COMPAT_BASELINE_/p')\"", baseline)
	}
	if len(floating) > 0 {
		t.Logf("compat: %d of %d baselines are a floating :main (%s) -- those "+
			"services have cut no release, so they contribute little skew; the "+
			"skew under test comes from %s",
			len(floating), len(fleet), strings.Join(floating, " "),
			strings.Join(releases, " "))
	}

	for _, upgraded := range []string{"piri", "ingot"} {
		t.Run("upgrade_"+upgraded, func(t *testing.T) {
			// A service outside the fleet makes otherThan exclude every member,
			// so nothing is built from HEAD and the subtest passes having
			// compared released against released. Not reachable with this
			// repository's go.work; reachable with a narrowed one, which
			// pkg/workspace's own error text tells people to make.
			if !slices.Contains(fleet, upgraded) {
				t.Skipf("%s is not in the active workspace (%v), so there is "+
					"nothing to upgrade it from HEAD", upgraded, fleet)
			}
			opts := []stack.Option{
				stack.WithPublishedImages(),
				stack.WithPiriNodes(stack.PiriNodeConfig{Postgres: true}),
			}
			// Everything from the released baseline set...
			for _, svc := range fleet {
				opts = append(opts, pinFor[svc](imageFor(t, svc, baseline[svc])))
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
