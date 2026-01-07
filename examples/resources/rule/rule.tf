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

resource "flipt_rule" "example" {
  namespace_key = "default"
  flag_key      = "my-feature"
  segment_key   = "premium-users"
  rank          = 1
}

# Import existing rule
# terraform import flipt_rule.example rule-id
