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

func TestAccSegmentDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSegmentDataSourceConfig("local", "test-namespace", "test-segment"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.flipt_segment.test", "environment_key", "local"),
					resource.TestCheckResourceAttr("data.flipt_segment.test", "namespace_key", "test-namespace"),
					resource.TestCheckResourceAttr("data.flipt_segment.test", "key", "test-segment"),
					resource.TestCheckResourceAttrSet("data.flipt_segment.test", "name"),
					resource.TestCheckResourceAttrSet("data.flipt_segment.test", "match_type"),
				),
			},
		},
	})
}

func testAccSegmentDataSourceConfig(envKey, namespaceKey, key string) string {
	return `
provider "flipt" {
  endpoint = "http://localhost:8080"
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
  name            = "Test Segment"
  match_type      = "ALL_MATCH_TYPE"
}

data "flipt_segment" "test" {
  environment_key = "` + envKey + `"
  namespace_key   = flipt_namespace.test.key
  key             = flipt_segment.test.key
  depends_on      = [flipt_segment.test]
}
`
}

func TestSegmentDataSourceHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
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
		}
	}))
	defer server.Close()

	if server.URL == "" {
		t.Fatal("Expected server URL to be set")
	}
}
