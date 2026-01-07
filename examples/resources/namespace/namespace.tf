terraform {
  required_providers {
    flipt = {
      source = "lerentis/flipt"
    }
  }
}

provider "flipt" {
  endpoint = "http://localhost:8080"
}

resource "flipt_namespace" "example" {
  key         = "production"
  name        = "Production Environment"
  description = "Namespace for production feature flags"
  protected   = true
}

resource "flipt_namespace" "staging" {
  key         = "staging"
  name        = "Staging Environment"
  description = "Namespace for staging feature flags"
  protected   = false
}
