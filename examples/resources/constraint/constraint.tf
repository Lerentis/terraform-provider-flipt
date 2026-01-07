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

resource "flipt_constraint" "example" {
  namespace_key = "default"
  segment_key   = "premium-users"
  type          = "STRING_COMPARISON_TYPE"
  property      = "subscription_tier"
  operator      = "eq"
  value         = "premium"
  description   = "Match premium subscription tier"
}

# Import existing constraint
# terraform import flipt_constraint.example constraint-id
