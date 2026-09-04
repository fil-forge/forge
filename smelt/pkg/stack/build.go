package stack

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// BuildImage builds a Docker image from the Dockerfile at the root of the
// given directory, with that directory as the build context. Every service —
// in-repo (piri, hilt, sprue, ingot) or external (guppy, indexing-service,
// delegator) — builds this way: each service directory is a self-contained
// build context.
// Returns the image tag. The image is automatically cleaned up when the test completes.
//
// This enables testing local code changes against the full smelt stack:
//
//	func TestWithLocalChanges(t *testing.T) {
//	    localPiri := stack.BuildImage(t, "../piri", "local-piri") // from smelt/
//	    s := stack.MustNewStack(t, stack.WithPiriImage(localPiri))
//	    // ... test against local changes
//	}
func BuildImage(t *testing.T, repoPath string, imageName string) string {
	t.Helper()

	// Create unique tag for this test run
	tag := fmt.Sprintf("%s:smelt-test-%d", imageName, time.Now().UnixNano())

	t.Logf("Building Docker image %s from %s...", tag, repoPath)

	cmd := exec.Command("docker", "build", "-t", tag, ".")
	cmd.Dir = repoPath
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

// buildForgeImage builds an in-repo forge service via the shared
// docker/Dockerfile (the monorepo has no per-service Dockerfiles). The build
// context is the repository root — the services reach the shared modules
// through replace directives, which a per-service context could not see — and
// the Dockerfile is parameterized by the SERVICE build arg (plus any extra
// --build-arg pairs, e.g. piri's BUILD_TAGS=skiff).
func buildForgeImage(t *testing.T, serviceDir, service, imageName string, extraArgs ...string) string {
	t.Helper()

	tag := fmt.Sprintf("%s:smelt-test-%d", imageName, time.Now().UnixNano())
	args := []string{"build",
		"-f", filepath.Join(serviceDir, "..", "docker", "Dockerfile"),
		"--build-arg", "SERVICE=" + service,
		"--target", "prod",
		"-t", tag,
	}
	for _, a := range extraArgs {
		args = append(args, "--build-arg", a)
	}
	args = append(args, filepath.Join(serviceDir, ".."))

	t.Logf("Building Docker image %s for %s from %s...", tag, service, serviceDir)
	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build Docker image: %v", err)
	}
	t.Cleanup(func() {
		t.Logf("Cleaning up Docker image %s", tag)
		_ = exec.Command("docker", "rmi", tag).Run()
	})
	return tag
}

// BuildPiriImage builds piri from its service directory and returns the image
// tag. The image is automatically cleaned up when the test completes.
//
// Example:
//
//	func TestWithLocalPiri(t *testing.T) {
//	    localPiri := stack.BuildPiriImage(t, "../piri") // from smelt/
//	    s := stack.MustNewStack(t, stack.WithPiriImage(localPiri))
//	}
func BuildPiriImage(t *testing.T, repoPath string) string {
	t.Helper()
	return buildForgeImage(t, repoPath, "piri", "local-piri", "BUILD_TAGS=skiff")
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

// BuildUploadImage builds the upload service (sprue) from its service
// directory and returns the image tag.
// The image is automatically cleaned up when the test completes.
func BuildUploadImage(t *testing.T, repoPath string) string {
	t.Helper()
	return buildForgeImage(t, repoPath, "sprue", "local-upload")
}
