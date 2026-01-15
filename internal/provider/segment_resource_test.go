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

func TestAccSegmentResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccSegmentResourceConfig("default", "test-namespace", "test-segment", "Test Segment", "ALL_MATCH_TYPE"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("flipt_segment.test", "environment_key", "default"),
					resource.TestCheckResourceAttr("flipt_segment.test", "namespace_key", "test-namespace"),
					resource.TestCheckResourceAttr("flipt_segment.test", "key", "test-segment"),
					resource.TestCheckResourceAttr("flipt_segment.test", "name", "Test Segment"),
					resource.TestCheckResourceAttr("flipt_segment.test", "match_type", "ALL_MATCH_TYPE"),
				),
			},
			// Update and Read testing
			{
				Config: testAccSegmentResourceConfig("default", "test-namespace", "test-segment", "Updated Segment", "ANY_MATCH_TYPE"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("flipt_segment.test", "name", "Updated Segment"),
					resource.TestCheckResourceAttr("flipt_segment.test", "match_type", "ANY_MATCH_TYPE"),
				),
			},
		},
	})
}

func testAccSegmentResourceConfig(envKey, namespaceKey, key, name, matchType string) string {
	return `
provider "flipt" {
  endpoint = "` + getTestFliptEndpoint() + `"
}

resource "flipt_namespace" "test" {
  environment_key = "` + envKey + `"
  key             = "` + namespaceKey + `"
  name            = "Test Namespace"
}

resource "flipt_segment" "test" {
  environment_key = "` + envKey + `"
  namespace_key   = flipt_namespace.test.key
  key             = "` + key + `"
  name            = "` + name + `"
  match_type      = "` + matchType + `"
}
`
}

func TestSegmentResourceHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.WriteHeader(http.StatusOK)
			response := map[string]interface{}{
				"resource": map[string]interface{}{
					"namespaceKey": "test-ns",
					"key":          "test-segment",
					"payload": map[string]interface{}{
						"@type":       "flipt.core.Segment",
						"key":         "test-segment",
						"name":        "Test Segment",
						"description": "",
						"matchType":   "ALL_MATCH_TYPE",
						"constraints": []interface{}{},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(response)
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			response := map[string]interface{}{
				"resource": map[string]interface{}{
					"namespaceKey": "test-ns",
					"key":          "test-segment",
					"payload": map[string]interface{}{
						"@type":       "flipt.core.Segment",
						"key":         "test-segment",
						"name":        "Test Segment",
						"description": "",
						"matchType":   "ALL_MATCH_TYPE",
						"constraints": []interface{}{},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(response)
		case http.MethodPut:
			w.WriteHeader(http.StatusOK)
			response := map[string]interface{}{
				"resource": map[string]interface{}{
					"namespaceKey": "test-ns",
					"key":          "test-segment",
					"payload": map[string]interface{}{
						"@type":       "flipt.core.Segment",
						"key":         "test-segment",
						"name":        "Updated Segment",
						"description": "",
						"matchType":   "ANY_MATCH_TYPE",
						"constraints": []interface{}{},
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
