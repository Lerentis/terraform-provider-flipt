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

func TestAccRuleResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccRuleResourceConfig("default", "test-namespace", "test-flag", "test-segment", "OR_SEGMENT_OPERATOR"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("flipt_rule.test", "environment_key", "default"),
					resource.TestCheckResourceAttr("flipt_rule.test", "namespace_key", "test-namespace"),
					resource.TestCheckResourceAttr("flipt_rule.test", "flag_key", "test-flag"),
					resource.TestCheckResourceAttr("flipt_rule.test", "segment_operator", "OR_SEGMENT_OPERATOR"),
				),
			},
			// Update and Read testing
			{
				Config: testAccRuleResourceConfig("default", "test-namespace", "test-flag", "test-segment", "AND_SEGMENT_OPERATOR"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("flipt_rule.test", "segment_operator", "AND_SEGMENT_OPERATOR"),
				),
			},
		},
	})
}

func testAccRuleResourceConfig(envKey, namespaceKey, flagKey, segmentKey, operator string) string {
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
  key             = "` + flagKey + `"
  name            = "Test Flag"
  type            = "VARIANT_FLAG_TYPE"
}

resource "flipt_segment" "test" {
  environment_key = "` + envKey + `"
  namespace_key   = flipt_namespace.test.key
  key             = "` + segmentKey + `"
  name            = "Test Segment"
  match_type      = "ALL_MATCH_TYPE"
}

resource "flipt_rule" "test" {
  environment_key  = "` + envKey + `"
  namespace_key    = flipt_namespace.test.key
  flag_key         = flipt_flag.test.key
  segment_keys     = [flipt_segment.test.key]
  segment_operator = "` + operator + `"
  rank             = 0
}
`
}

func TestRuleResourceHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Return flag with rules
			w.WriteHeader(http.StatusOK)
			response := map[string]interface{}{
				"resource": map[string]interface{}{
					"namespaceKey": "test-ns",
					"key":          "test-flag",
					"payload": map[string]interface{}{
						"@type":       "flipt.core.Flag",
						"key":         "test-flag",
						"name":        "Test Flag",
						"type":        "VARIANT_FLAG_TYPE",
						"enabled":     true,
						"description": "",
						"variants":    []interface{}{},
						"rules": []interface{}{
							map[string]interface{}{
								"id": "test-rule-id",
								"segments": []interface{}{
									map[string]interface{}{
										"segmentKey": "test-segment",
									},
								},
								"segmentOperator": "OR_SEGMENT_OPERATOR",
								"rank":            0,
							},
						},
						"metadata": map[string]interface{}{},
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
						"name":        "Test Flag",
						"type":        "VARIANT_FLAG_TYPE",
						"enabled":     true,
						"description": "",
						"variants":    []interface{}{},
						"rules": []interface{}{
							map[string]interface{}{
								"id": "test-rule-id",
								"segments": []interface{}{
									map[string]interface{}{
										"segmentKey": "test-segment",
									},
								},
								"segmentOperator": "AND_SEGMENT_OPERATOR",
								"rank":            0,
							},
						},
						"metadata": map[string]interface{}{},
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
