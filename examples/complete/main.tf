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
resource "flipt_namespace" "production" {
  key         = "production"
  name        = "Production Environment"
  description = "Namespace for production feature flags"
}

# Create a feature flag
resource "flipt_flag" "new_ui" {
  namespace_key = flipt_namespace.production.key
  key           = "new-ui"
  name          = "New UI"
  description   = "Enable the new user interface"
  enabled       = true
  type          = "VARIANT_FLAG_TYPE"
}

# Create variants for the flag
resource "flipt_variant" "ui_blue" {
  namespace_key = flipt_namespace.production.key
  flag_key      = flipt_flag.new_ui.key
  key           = "blue"
  name          = "Blue Theme"
  description   = "Blue color scheme"
  attachment    = jsonencode({ theme = "blue", font = "sans-serif" })
}

resource "flipt_variant" "ui_green" {
  namespace_key = flipt_namespace.production.key
  flag_key      = flipt_flag.new_ui.key
  key           = "green"
  name          = "Green Theme"
  description   = "Green color scheme"
  attachment    = jsonencode({ theme = "green", font = "serif" })
}

# Create a segment
resource "flipt_segment" "beta_users" {
  namespace_key = flipt_namespace.production.key
  key           = "beta-users"
  name          = "Beta Users"
  description   = "Users enrolled in beta program"
  match_type    = "ALL_MATCH_TYPE"
}

# Add constraints to the segment
resource "flipt_constraint" "beta_user_email" {
  namespace_key = flipt_namespace.production.key
  segment_key   = flipt_segment.beta_users.key
  type          = "STRING_COMPARISON_TYPE"
  property      = "email"
  operator      = "suffix"
  value         = "@beta.example.com"
  description   = "Match beta email addresses"
}

# Create a rule linking the flag and segment
resource "flipt_rule" "beta_ui_rule" {
  namespace_key = flipt_namespace.production.key
  flag_key      = flipt_flag.new_ui.key
  segment_key   = flipt_segment.beta_users.key
  rank          = 1
}

# Boolean flag example
resource "flipt_flag" "maintenance_mode" {
  namespace_key = flipt_namespace.production.key
  key           = "maintenance-mode"
  name          = "Maintenance Mode"
  description   = "Enable maintenance mode"
  enabled       = false
  type          = "BOOLEAN_FLAG_TYPE"
}
