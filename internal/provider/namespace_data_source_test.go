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

func TestAccNamespaceDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccNamespaceDataSourceConfig("local", "test-namespace"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.flipt_namespace.test", "environment_key", "local"),
					resource.TestCheckResourceAttr("data.flipt_namespace.test", "key", "test-namespace"),
					resource.TestCheckResourceAttrSet("data.flipt_namespace.test", "name"),
				),
			},
		},
	})
}

func testAccNamespaceDataSourceConfig(envKey, key string) string {
	return `
resource "flipt_namespace" "test" {
  environment_key = "` + envKey + `"
  key             = "` + key + `"
  name            = "Test Namespace"
}

data "flipt_namespace" "test" {
  environment_key = "` + envKey + `"
  key             = flipt_namespace.test.key
  depends_on      = [flipt_namespace.test]
}
`
}

func TestNamespaceDataSourceHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
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
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	if server.URL == "" {
		t.Fatal("Expected server URL to be set")
	}
}
