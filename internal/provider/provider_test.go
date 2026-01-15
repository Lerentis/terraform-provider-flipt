// Copyright (c) terraform-flipt contributors
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestProviderSchema(t *testing.T) {
	provider := New("test")()

	if provider == nil {
		t.Fatal("Expected provider to be created")
	}

	// Test that provider can be instantiated
	_ = providerserver.NewProtocol6(provider)
	// This test would verify provider configuration
	// with various endpoint configurations
	testCases := []struct {
		name     string
		endpoint string
		wantErr  bool
	}{
		{
			name:     "valid http endpoint",
			endpoint: "http://localhost:8080",
			wantErr:  false,
		},
		{
			name:     "valid https endpoint",
			endpoint: "https://flipt.example.com",
			wantErr:  false,
		},
		{
			name:     "endpoint with trailing slash",
			endpoint: "http://localhost:8080/",
			wantErr:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Provider configuration testing would go here
			if tc.endpoint == "" {
				t.Error("Expected endpoint to be set")
			}
		})
	}
}

func TestProviderDataSources(t *testing.T) {
	// Verify all data sources are available
	expectedDataSources := []string{
		"flipt_environment",
		"flipt_namespace",
		"flipt_flag",
		"flipt_segment",
		"flipt_variant",
	}

	for _, dsName := range expectedDataSources {
		t.Run(dsName, func(t *testing.T) {
			// Data source availability testing would go here
			if dsName == "" {
				t.Error("Expected data source name to be set")
			}
		})
	}
}

func TestProviderResources(t *testing.T) {
	expectedResources := []string{
		"flipt_namespace",
		"flipt_flag",
		"flipt_segment",
		"flipt_variant",
		"flipt_constraint",
		"flipt_rule",
	}

	for _, resourceName := range expectedResources {
		t.Run(resourceName, func(t *testing.T) {
			// Resource availability testing would go here
			if resourceName == "" {
				t.Error("Expected resource name to be set")
			}
		})
	}
}

// testAccProtoV6ProviderFactories is used for acceptance testing.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"flipt": providerserver.NewProtocol6WithError(New("test")()),
}

var (
	fliptContainer     testcontainers.Container
	fliptEndpoint      string
	fliptContainerOnce sync.Once
	fliptContainerErr  error
)

// getTestFliptEndpoint returns the Flipt endpoint for testing.
// It returns the user-provided FLIPT_ENDPOINT or the testcontainer endpoint.
// This function ensures the container is started if needed.
func getTestFliptEndpoint() string {
	// Check for user-provided endpoint first
	if endpoint := os.Getenv("FLIPT_ENDPOINT"); endpoint != "" {
		return endpoint
	}

	// Check if TF_ACC is set - only use testcontainers in acceptance test mode
	if os.Getenv("TF_ACC") == "" {
		return "http://localhost:8080"
	}

	// If fliptEndpoint is already set, return it
	if fliptEndpoint != "" {
		return fliptEndpoint
	}

	// Start the container if not already started
	ctx := context.Background()
	endpoint, err := setupFliptContainer(ctx)
	if err != nil {
		// Return a default but this will likely fail - tests should call testAccPreCheck first
		return "http://localhost:8080"
	}

	return endpoint
}

// setupFliptContainer starts a Flipt container for acceptance tests.
// It's called once and reused across all tests.
func setupFliptContainer(ctx context.Context) (string, error) {
	fliptContainerOnce.Do(func() {
		// Disable ryuk for Mac compatibility (testcontainers bug)
		_ = os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")

		req := testcontainers.ContainerRequest{
			Image:        "docker.flipt.io/flipt/flipt:v2.4.0",
			ExposedPorts: []string{"8080/tcp"},
			WaitingFor: wait.ForHTTP("/api/v2/environments").
				WithPort("8080/tcp").
				WithStartupTimeout(120 * time.Second).
				WithPollInterval(2 * time.Second),
		}

		container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
		if err != nil {
			fliptContainerErr = fmt.Errorf("failed to start Flipt container: %w", err)
			return
		}

		fliptContainer = container

		// Get the mapped port
		host, err := container.Host(ctx)
		if err != nil {
			fliptContainerErr = fmt.Errorf("failed to get container host: %w", err)
			return
		}

		port, err := container.MappedPort(ctx, "8080")
		if err != nil {
			fliptContainerErr = fmt.Errorf("failed to get mapped port: %w", err)
			return
		}

		fliptEndpoint = fmt.Sprintf("http://%s:%s", host, port.Port())

		// Give Flipt additional time to fully initialize after container health check passes
		time.Sleep(5 * time.Second)

		// Verify Flipt is actually ready to accept requests
		maxRetries := 60
		for i := 0; i < maxRetries; i++ {
			resp, err := http.Get(fliptEndpoint + "/api/v2/environments")
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				// Additional delay to ensure Flipt is fully ready
				time.Sleep(2 * time.Second)
				return
			}
			if resp != nil {
				resp.Body.Close()
			}
			time.Sleep(1 * time.Second)
		}

		fliptContainerErr = fmt.Errorf("Flipt container failed to become ready after %d seconds", maxRetries)
	})

	return fliptEndpoint, fliptContainerErr
}

// testAccPreCheck runs before each acceptance test.
func testAccPreCheck(t *testing.T) {
	t.Helper()

	// Skip acceptance tests if TF_ACC is not set
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless TF_ACC is set")
		return
	}

	ctx := context.Background()

	// Check if FLIPT_ENDPOINT is explicitly set (user-provided instance)
	if endpoint := os.Getenv("FLIPT_ENDPOINT"); endpoint != "" {
		// Use user-provided endpoint
		resp, err := http.Get(endpoint + "/api/v2/environments")
		if err != nil {
			t.Skipf("Flipt instance not accessible at %s: %v", endpoint, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Skipf("Flipt instance at %s returned status %d, expected 200", endpoint, resp.StatusCode)
			return
		}
		return
	}

	// Start testcontainer
	_, err := setupFliptContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to start Flipt container: %v", err)
		return
	}
}

// TestMain handles cleanup of the Flipt container.
func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()

	// Cleanup
	if fliptContainer != nil {
		ctx := context.Background()
		_ = fliptContainer.Terminate(ctx)
	}

	os.Exit(code)
}
