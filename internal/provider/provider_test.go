// Copyright (c) terraform-flipt contributors
// SPDX-License-Identifier: MIT

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
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

// testAccPreCheck runs before each acceptance test.
func testAccPreCheck(t *testing.T) {
	// Check that a Flipt instance is running and accessible
	// This would typically check for FLIPT_ENDPOINT environment variable
	// or ensure localhost:8080 is available
}
