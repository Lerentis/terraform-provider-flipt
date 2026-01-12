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
  description     = "Example namespace for constraints"
}

# Create a segment
resource "flipt_segment" "premium_users" {
  namespace_key = flipt_namespace.example.key
  key           = "premium-users"
  name          = "Premium Users"
  description   = "Users with premium subscription"
  match_type    = "ALL_MATCH_TYPE"
}

# Add email constraint
resource "flipt_constraint" "premium_email" {
  namespace_key = flipt_namespace.example.key
  segment_key   = flipt_segment.premium_users.key
  property      = "email"
  type          = "STRING_COMPARISON_TYPE"
  operator      = "suffix"
  value         = "@premium.example.com"
  description   = "Match premium email domain"
}

# Add subscription tier constraint
resource "flipt_constraint" "premium_tier" {
  namespace_key = flipt_namespace.example.key
  segment_key   = flipt_segment.premium_users.key
  property      = "subscription_tier"
  type          = "STRING_COMPARISON_TYPE"
  operator      = "eq"
  value         = "premium"
  description   = "Match premium subscription tier"
}

# Note: Import example (not used in this config)
# terraform import flipt_constraint.premium_tier default/premium-users/subscription_tier

