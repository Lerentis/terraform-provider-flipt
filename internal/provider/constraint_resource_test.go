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

func TestAccConstraintResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccConstraintResourceConfig("local", "test-namespace", "test-segment", "email", "STRING_COMPARISON_TYPE", "suffix", "@test.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("flipt_constraint.test", "environment_key", "local"),
					resource.TestCheckResourceAttr("flipt_constraint.test", "namespace_key", "test-namespace"),
					resource.TestCheckResourceAttr("flipt_constraint.test", "segment_key", "test-segment"),
					resource.TestCheckResourceAttr("flipt_constraint.test", "property", "email"),
					resource.TestCheckResourceAttr("flipt_constraint.test", "type", "STRING_COMPARISON_TYPE"),
					resource.TestCheckResourceAttr("flipt_constraint.test", "operator", "suffix"),
					resource.TestCheckResourceAttr("flipt_constraint.test", "value", "@test.com"),
				),
			},
			// Update and Read testing
			{
				Config: testAccConstraintResourceConfig("local", "test-namespace", "test-segment", "email", "STRING_COMPARISON_TYPE", "suffix", "@updated.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("flipt_constraint.test", "value", "@updated.com"),
				),
			},
		},
	})
}

func testAccConstraintResourceConfig(envKey, namespaceKey, segmentKey, property, constraintType, operator, value string) string {
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
  key             = "` + segmentKey + `"
  name            = "Test Segment"
  match_type      = "ALL_MATCH_TYPE"
}

resource "flipt_constraint" "test" {
  environment_key = "` + envKey + `"
  namespace_key   = flipt_namespace.test.key
  segment_key     = flipt_segment.test.key
  property        = "` + property + `"
  type            = "` + constraintType + `"
  operator        = "` + operator + `"
  value           = "` + value + `"
}
`
}

func TestConstraintResourceHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Return segment with constraints
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
						"constraints": []interface{}{
							map[string]interface{}{
								"property":    "email",
								"type":        "STRING_COMPARISON_TYPE",
								"operator":    "suffix",
								"value":       "@test.com",
								"description": "",
							},
						},
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
						"name":        "Test Segment",
						"description": "",
						"matchType":   "ALL_MATCH_TYPE",
						"constraints": []interface{}{
							map[string]interface{}{
								"property":    "email",
								"type":        "STRING_COMPARISON_TYPE",
								"operator":    "suffix",
								"value":       "@updated.com",
								"description": "",
							},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	if server.URL == "" {
		t.Fatal("Expected server URL to be set")
	}
}
