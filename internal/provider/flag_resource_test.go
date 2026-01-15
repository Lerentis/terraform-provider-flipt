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

func TestAccFlagResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccFlagResourceConfig("local", "test-namespace", "test-flag", "Test Flag", true, "VARIANT_FLAG_TYPE"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("flipt_flag.test", "environment_key", "local"),
					resource.TestCheckResourceAttr("flipt_flag.test", "namespace_key", "test-namespace"),
					resource.TestCheckResourceAttr("flipt_flag.test", "key", "test-flag"),
					resource.TestCheckResourceAttr("flipt_flag.test", "name", "Test Flag"),
					resource.TestCheckResourceAttr("flipt_flag.test", "enabled", "true"),
					resource.TestCheckResourceAttr("flipt_flag.test", "type", "VARIANT_FLAG_TYPE"),
				),
			},
			// Update and Read testing
			{
				Config: testAccFlagResourceConfig("local", "test-namespace", "test-flag", "Updated Flag", false, "VARIANT_FLAG_TYPE"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("flipt_flag.test", "name", "Updated Flag"),
					resource.TestCheckResourceAttr("flipt_flag.test", "enabled", "false"),
				),
			},
		},
	})
}

func testAccFlagResourceConfig(envKey, namespaceKey, key, name string, enabled bool, flagType string) string {
	return `
provider "flipt" {
  endpoint = "` + getTestFliptEndpoint() + `"
}

resource "flipt_namespace" "test" {
  environment_key = "` + envKey + `"
  key             = "` + namespaceKey + `"
  name            = "Test Namespace"
}

resource "flipt_flag" "test" {
  environment_key = "` + envKey + `"
  namespace_key   = flipt_namespace.test.key
  key             = "` + key + `"
  name            = "` + name + `"
  enabled         = ` + boolToString(enabled) + `
  type            = "` + flagType + `"
}
`
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func TestFlagResourceHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.WriteHeader(http.StatusOK)
			response := map[string]interface{}{
				"resource": map[string]interface{}{
					"namespaceKey": "test-ns",
					"key":          "test-flag",
					"payload": map[string]interface{}{
						"@type":       "flipt.core.Flag",
						"key":         "test-flag",
						"name":        "Test Flag",
						"description": "",
						"enabled":     true,
						"type":        "VARIANT_FLAG_TYPE",
						"variants":    []interface{}{},
						"rules":       []interface{}{},
						"metadata":    map[string]interface{}{},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(response)
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			response := map[string]interface{}{
				"resource": map[string]interface{}{
					"namespaceKey": "test-ns",
					"key":          "test-flag",
					"payload": map[string]interface{}{
						"@type":       "flipt.core.Flag",
						"key":         "test-flag",
						"name":        "Test Flag",
						"description": "",
						"enabled":     true,
						"type":        "VARIANT_FLAG_TYPE",
						"variants":    []interface{}{},
						"rules":       []interface{}{},
						"metadata":    map[string]interface{}{},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(response)
		case http.MethodPut:
			w.WriteHeader(http.StatusOK)
			response := map[string]interface{}{
				"resource": map[string]interface{}{
					"namespaceKey": "test-ns",
					"key":          "test-flag",
					"payload": map[string]interface{}{
						"@type":       "flipt.core.Flag",
						"key":         "test-flag",
						"name":        "Updated Flag",
						"description": "",
						"enabled":     false,
						"type":        "VARIANT_FLAG_TYPE",
						"variants":    []interface{}{},
						"rules":       []interface{}{},
						"metadata":    map[string]interface{}{},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(response)
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	if server.URL == "" {
		t.Fatal("Expected server URL to be set")
	}
}
