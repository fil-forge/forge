package stack

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

// BuildImage builds a Docker image from a repo whose Dockerfile sits at its
// root, with that repo as the build context. This is the shape of the services
// that live OUTSIDE the forge monorepo — guppy, indexing-service, delegator.
//
// For the services inside the monorepo (piri, sprue, hilt, ingot) use the
// per-service helpers below: their Dockerfiles take the monorepo root as the
// build context, so they need an explicit -f.
//
//	func TestWithLocalChanges(t *testing.T) {
//	    localGuppy := stack.BuildImage(t, "../../guppy", "local-guppy")
//	    s := stack.MustNewStack(t, stack.WithGuppyImage(localGuppy))
//	}
func BuildImage(t *testing.T, repoPath string, imageName string) string {
	t.Helper()
	return buildImage(t, repoPath, "", imageName)
}

// BuildMonorepoImage builds an in-monorepo service image. rootPath is the
// monorepo root (the directory holding piri/, hilt/, sprue/, ingot/, smelt/)
// and dockerfile is the path to the service's Dockerfile relative to it.
//
// The root is the build context because ingot's go.mod resolves hilt and smelt
// through replace directives at ../hilt and ../smelt — those siblings have to
// be inside the context. The other services don't strictly need the root, but
// they use it too so every forge service builds the same way.
func BuildMonorepoImage(t *testing.T, rootPath, dockerfile, imageName string) string {
	t.Helper()
	return buildImage(t, rootPath, dockerfile, imageName)
}

// buildImage runs the docker build. An empty dockerfile means "use the default
// Dockerfile at the context root".
//
// Note there is no --target: the built stage is whichever the Dockerfile ends
// with. For piri that is `prod`; for hilt/sprue/ingot the final stage is `dev`
// (debug tooling + dlv). That has always been this helper's behavior — pass a
// service's prod image explicitly if you need the slim one.
func buildImage(t *testing.T, contextPath, dockerfile, imageName string) string {
	t.Helper()

	// Create unique tag for this test run
	tag := fmt.Sprintf("%s:smelt-test-%d", imageName, time.Now().UnixNano())

	args := []string{"build", "-t", tag}
	if dockerfile != "" {
		args = append(args, "-f", dockerfile)
	}
	args = append(args, ".")

	t.Logf("Building Docker image %s from %s (dockerfile=%q)...", tag, contextPath, dockerfile)

	cmd := exec.Command("docker", args...)
	cmd.Dir = contextPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build Docker image: %v", err)
	}

	// Cleanup image after test
	t.Cleanup(func() {
		t.Logf("Cleaning up Docker image %s", tag)
		_ = exec.Command("docker", "rmi", tag).Run()
	})

	t.Logf("Successfully built image: %s", tag)
	return tag
}

// BuildPiriImage builds piri from the monorepo and returns the image tag.
// rootPath is the monorepo root. The image is cleaned up when the test ends.
//
// Example:
//
//	func TestWithLocalPiri(t *testing.T) {
//	    localPiri := stack.BuildPiriImage(t, "..") // from smelt/, .. is the root
//	    s := stack.MustNewStack(t, stack.WithPiriImage(localPiri))
//	}
//
// Building an image is the slow path. Prefer WithServiceBinary (or
// WithWorkspaceBinaries) to mount a host-built binary over the published image
// — that is what CI does, and it takes seconds rather than minutes.
func BuildPiriImage(t *testing.T, rootPath string) string {
	t.Helper()
	return BuildMonorepoImage(t, rootPath, "piri/Dockerfile", "local-piri")
}

// BuildHiltImage builds hilt from the monorepo and returns the image tag.
func BuildHiltImage(t *testing.T, rootPath string) string {
	t.Helper()
	return BuildMonorepoImage(t, rootPath, "hilt/Dockerfile", "local-hilt")
}

// BuildIngotImage builds ingot from the monorepo and returns the image tag.
func BuildIngotImage(t *testing.T, rootPath string) string {
	t.Helper()
	return BuildMonorepoImage(t, rootPath, "ingot/Dockerfile", "local-ingot")
}

// BuildGuppyImage builds guppy from a local repo and returns the image tag.
// The image is automatically cleaned up when the test completes.
//
// Example:
//
//	func TestWithLocalGuppy(t *testing.T) {
//	    localGuppy := stack.BuildGuppyImage(t, "..")
//	    s := stack.MustNewStack(t, stack.WithGuppyImage(localGuppy))
//	}
func BuildGuppyImage(t *testing.T, repoPath string) string {
	t.Helper()
	return BuildImage(t, repoPath, "local-guppy")
}

// BuildIndexerImage builds the indexing-service from a local repo and returns the image tag.
// The image is automatically cleaned up when the test completes.
func BuildIndexerImage(t *testing.T, repoPath string) string {
	t.Helper()
	return BuildImage(t, repoPath, "local-indexer")
}

// BuildDelegatorImage builds the delegator from a local repo and returns the image tag.
// The image is automatically cleaned up when the test completes.
func BuildDelegatorImage(t *testing.T, repoPath string) string {
	t.Helper()
	return BuildImage(t, repoPath, "local-delegator")
}

// BuildUploadImage builds the upload service (sprue) from the monorepo and
// returns the image tag. rootPath is the monorepo root.
func BuildUploadImage(t *testing.T, rootPath string) string {
	t.Helper()
	return BuildMonorepoImage(t, rootPath, "sprue/Dockerfile", "local-upload")
}
