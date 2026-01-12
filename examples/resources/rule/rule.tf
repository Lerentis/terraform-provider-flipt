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

# Create a namespace
resource "flipt_namespace" "example" {
  environment_key = "default"
  key             = "example"
  name            = "Example Namespace"
}

# Create a flag
resource "flipt_flag" "my_feature" {
  namespace_key = flipt_namespace.example.key
  key           = "my-feature"
  name          = "My Feature"
  type          = "VARIANT_FLAG_TYPE"
  enabled       = true
}

# Create a segment
resource "flipt_segment" "premium_users" {
  namespace_key = flipt_namespace.example.key
  key           = "premium-users"
  name          = "Premium Users"
  match_type    = "ALL_MATCH_TYPE"
}

# Create a rule linking the flag and segment(s)
resource "flipt_rule" "example" {
  namespace_key    = flipt_namespace.example.key
  flag_key         = flipt_flag.my_feature.key
  segment_keys     = [flipt_segment.premium_users.key]
  segment_operator = "OR_SEGMENT_OPERATOR"
  rank             = 0
}

# Note: Import example (not used in this config)
# terraform import flipt_rule.example namespace_key/flag_key/rule_id

