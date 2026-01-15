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

func TestAccVariantDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVariantDataSourceConfig("default", "test-namespace", "test-flag", "test-variant"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.flipt_variant.test", "environment_key", "default"),
					resource.TestCheckResourceAttr("data.flipt_variant.test", "namespace_key", "test-namespace"),
					resource.TestCheckResourceAttr("data.flipt_variant.test", "flag_key", "test-flag"),
					resource.TestCheckResourceAttr("data.flipt_variant.test", "key", "test-variant"),
					resource.TestCheckResourceAttrSet("data.flipt_variant.test", "name"),
				),
			},
		},
	})
}

func testAccVariantDataSourceConfig(envKey, namespaceKey, flagKey, key string) string {
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

resource "flipt_variant" "test" {
  environment_key = "` + envKey + `"
  namespace_key   = flipt_namespace.test.key
  flag_key        = flipt_flag.test.key
  key             = "` + key + `"
  name            = "Test Variant"
}

data "flipt_variant" "test" {
  environment_key = "` + envKey + `"
  namespace_key   = flipt_namespace.test.key
  flag_key        = flipt_flag.test.key
  key             = flipt_variant.test.key
  depends_on      = [flipt_variant.test]
}
`
}

func TestVariantDataSourceHTTP(t *testing.T) {
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
						"type":        "VARIANT_FLAG_TYPE",
						"enabled":     true,
						"description": "",
						"variants": []interface{}{
							map[string]interface{}{
								"key":         "test-variant",
								"name":        "Test Variant",
								"description": "",
								"attachment":  "",
							},
						},
						"rules":    []interface{}{},
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
