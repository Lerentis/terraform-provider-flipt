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

resource "flipt_segment" "example" {
  namespace_key = "default"
  key           = "premium-users"
  name          = "Premium Users"
  description   = "Users with premium subscription"
  match_type    = "ANY_MATCH_TYPE"
}

# Import existing segment
# terraform import flipt_segment.example default/existing-segment-key
