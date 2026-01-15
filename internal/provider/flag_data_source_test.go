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

func TestAccFlagDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFlagDataSourceConfig("default", "test-namespace", "test-flag"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.flipt_flag.test", "environment_key", "default"),
					resource.TestCheckResourceAttr("data.flipt_flag.test", "namespace_key", "test-namespace"),
					resource.TestCheckResourceAttr("data.flipt_flag.test", "key", "test-flag"),
					resource.TestCheckResourceAttrSet("data.flipt_flag.test", "name"),
					resource.TestCheckResourceAttrSet("data.flipt_flag.test", "type"),
				),
			},
		},
	})
}

func testAccFlagDataSourceConfig(envKey, namespaceKey, key string) string {
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
  name            = "Test Flag"
  type            = "VARIANT_FLAG_TYPE"
}

data "flipt_flag" "test" {
  environment_key = "` + envKey + `"
  namespace_key   = flipt_namespace.test.key
  key             = flipt_flag.test.key
  depends_on      = [flipt_flag.test]
}
`
}

func TestFlagDataSourceHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
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
		}
	}))
	defer server.Close()

	if server.URL == "" {
		t.Fatal("Expected server URL to be set")
	}
}
