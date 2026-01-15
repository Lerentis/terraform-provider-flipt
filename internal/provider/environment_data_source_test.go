// Copyright (c) terraform-provider-flipt contributors
// SPDX-License-Identifier: MIT

package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccEnvironmentDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEnvironmentDataSourceConfig("default"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.flipt_environment.test", "key", "default"),
					resource.TestCheckResourceAttrSet("data.flipt_environment.test", "name"),
				),
			},
		},
	})
}

func testAccEnvironmentDataSourceConfig(key string) string {
	return `
provider "flipt" {
  endpoint = "` + getTestFliptEndpoint() + `"
}

data "flipt_environment" "test" {
  key = "` + key + `"
}
`
}

func TestEnvironmentDataSourceHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v2/environments" {
			w.WriteHeader(http.StatusOK)
			response := map[string]interface{}{
				"environments": []interface{}{
					map[string]interface{}{
						"key":         "local",
						"name":        "Local",
						"description": "Local environment",
						"default":     true,
						"protected":   false,
					},
					map[string]interface{}{
						"key":         "staging",
						"name":        "Staging",
						"description": "Staging environment",
						"default":     false,
						"protected":   false,
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
