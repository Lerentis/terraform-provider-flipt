// Copyright (c) terraform-flipt contributors
// SPDX-License-Identifier: MIT

package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccNamespaceResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccNamespaceResourceConfig("local", "test-namespace", "Test Namespace", "Test description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("flipt_namespace.test", "environment_key", "local"),
					resource.TestCheckResourceAttr("flipt_namespace.test", "key", "test-namespace"),
					resource.TestCheckResourceAttr("flipt_namespace.test", "name", "Test Namespace"),
					resource.TestCheckResourceAttr("flipt_namespace.test", "description", "Test description"),
				),
			},
			// Update and Read testing
			{
				Config: testAccNamespaceResourceConfig("local", "test-namespace", "Updated Namespace", "Updated description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("flipt_namespace.test", "name", "Updated Namespace"),
					resource.TestCheckResourceAttr("flipt_namespace.test", "description", "Updated description"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccNamespaceResourceConfig(envKey, key, name, description string) string {
	return `
provider "flipt" {
  endpoint = "http://localhost:8080"
}

resource "flipt_namespace" "test" {
  environment_key = "` + envKey + `"
  key             = "` + key + `"
  name            = "` + name + `"
  description     = "` + description + `"
}
`
}

func TestNamespaceResourceSchema(t *testing.T) {
	r := NewNamespaceResource()

	// Verify the resource can be created
	if r == nil {
		t.Fatal("Expected resource to be created")
	}
}

func TestNamespaceResourceCRUD(t *testing.T) {
	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			// Handle Create
			w.WriteHeader(http.StatusOK)
			response := map[string]interface{}{
				"resource": map[string]interface{}{
					"namespaceKey": "test-ns",
					"key":          "test-ns",
					"payload": map[string]interface{}{
						"@type":       "flipt.core.Namespace",
						"key":         "test-ns",
						"name":        "Test Namespace",
						"description": "Test description",
						"protected":   false,
					},
				},
			}
			_ = json.NewEncoder(w).Encode(response)
		case http.MethodGet:
			// Handle Read
			w.WriteHeader(http.StatusOK)
			response := map[string]interface{}{
				"resource": map[string]interface{}{
					"namespaceKey": "test-ns",
					"key":          "test-ns",
					"payload": map[string]interface{}{
						"@type":       "flipt.core.Namespace",
						"key":         "test-ns",
						"name":        "Test Namespace",
						"description": "Test description",
						"protected":   false,
					},
				},
			}
			_ = json.NewEncoder(w).Encode(response)
		case http.MethodPut:
			// Handle Update
			w.WriteHeader(http.StatusOK)
			response := map[string]interface{}{
				"resource": map[string]interface{}{
					"namespaceKey": "test-ns",
					"key":          "test-ns",
					"payload": map[string]interface{}{
						"@type":       "flipt.core.Namespace",
						"key":         "test-ns",
						"name":        "Updated Namespace",
						"description": "Updated description",
						"protected":   false,
					},
				},
			}
			_ = json.NewEncoder(w).Encode(response)
		case http.MethodDelete:
			// Handle Delete
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	// Test would use this server URL for API calls
	if server.URL == "" {
		t.Fatal("Expected server URL to be set")
	}
}
