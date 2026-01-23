package integration

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Configure testcontainers for Podman before any tests run
	setupTestcontainersForPodman()

	// Run tests
	code := m.Run()

	os.Exit(code)
}

func setupTestcontainersForPodman() {
	// Check if we're using Podman (docker.sock is symlink to podman)
	if target, err := os.Readlink("/var/run/docker.sock"); err == nil && len(target) > 0 {
		// We're using Podman, configure environment for testcontainers

		// Use podman network instead of bridge
		os.Setenv("TESTCONTAINERS_RYUK_CONTAINER_PRIVILEGED", "true")

		// Create network if it doesn't exist (will be used by testcontainers)
		os.Setenv("TESTCONTAINERS_DOCKER_NETWORK", "k8s-controller-test")

		// Ensure Docker socket points to Podman
		os.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")

		// Testcontainers should use the Podman default network
		os.Setenv("TC_HOST", "")
	}
}
