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

resource "flipt_distribution" "example" {
  namespace_key = "default"
  flag_key      = "my-feature"
  rule_id       = "rule-id-here"
  variant_id    = "variant-id-here"
  rollout       = 50.0
}

# Import existing distribution
# terraform import flipt_distribution.example distribution-id
