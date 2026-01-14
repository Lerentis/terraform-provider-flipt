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

# ============================================
# DATA SOURCES - Test reading existing data
# ============================================

# Read environment information
data "flipt_environment" "default" {
  key = "local"
}

# Output environment details
output "environment_name" {
  value       = data.flipt_environment.default.name
  description = "Default environment name"
}

output "environment_is_default" {
  value       = data.flipt_environment.default.default
  description = "Whether this is the default environment"
}

# ============================================
# RESOURCES - Test creating resources
# ============================================

# Create a namespace
resource "flipt_namespace" "production" {
  environment_key = "local" # Explicitly set the environment
  key             = "production"
  name            = "Production Environment"
  description     = "Namespace for production feature flags"
}

# Read namespace data source (after creation)
data "flipt_namespace" "production_data" {
  environment_key = "local"
  key             = flipt_namespace.production.key
  depends_on      = [flipt_namespace.production]
}

output "namespace_protected" {
  value       = data.flipt_namespace.production_data.protected
  description = "Whether the namespace is protected"
}

# # Create a feature flag
resource "flipt_flag" "new_ui" {
  namespace_key = flipt_namespace.production.key
  key           = "new-ui"
  name          = "New UI"
  description   = "Enable the new user interface"
  enabled       = true
  type          = "VARIANT_FLAG_TYPE"
}

# Read flag data source (after creation)
data "flipt_flag" "new_ui_data" {
  namespace_key = flipt_namespace.production.key
  key           = flipt_flag.new_ui.key
  depends_on    = [flipt_flag.new_ui]
}

output "flag_type" {
  value       = data.flipt_flag.new_ui_data.type
  description = "Type of the flag"
}

output "flag_enabled" {
  value       = data.flipt_flag.new_ui_data.enabled
  description = "Whether the flag is enabled"
}

# # Create variants for the flag
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

# Read variant data source (after creation)
data "flipt_variant" "ui_blue_data" {
  namespace_key = flipt_namespace.production.key
  flag_key      = flipt_flag.new_ui.key
  key           = flipt_variant.ui_blue.key
  depends_on    = [flipt_variant.ui_blue]
}

output "variant_attachment" {
  value       = data.flipt_variant.ui_blue_data.attachment
  description = "Variant attachment JSON"
}

# Create a segment
resource "flipt_segment" "beta_users" {
  namespace_key = flipt_namespace.production.key
  key           = "beta-users"
  name          = "Beta Users"
  description   = "Users enrolled in beta program"
  match_type    = "ALL_MATCH_TYPE"
}

# Read segment data source (after creation)
data "flipt_segment" "beta_users_data" {
  namespace_key = flipt_namespace.production.key
  key           = flipt_segment.beta_users.key
  depends_on    = [flipt_segment.beta_users]
}

output "segment_match_type" {
  value       = data.flipt_segment.beta_users_data.match_type
  description = "Segment match type"
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
  namespace_key    = flipt_namespace.production.key
  flag_key         = flipt_flag.new_ui.key
  segment_keys     = [flipt_segment.beta_users.key]
  segment_operator = "OR_SEGMENT_OPERATOR"
  rank             = 0
}

# # Boolean flag example
resource "flipt_flag" "maintenance_mode" {
  namespace_key = flipt_namespace.production.key
  key           = "maintenance-mode"
  name          = "Maintenance Mode"
  description   = "Enable maintenance mode"
  enabled       = false
  type          = "BOOLEAN_FLAG_TYPE"
}

# ============================================
# ADDITIONAL TESTS
# ============================================

# Create another segment for OR operator testing
resource "flipt_segment" "premium_users" {
  namespace_key = flipt_namespace.production.key
  key           = "premium-users"
  name          = "Premium Users"
  description   = "Users with premium subscription"
  match_type    = "ANY_MATCH_TYPE"
}

# Add constraint for premium users
resource "flipt_constraint" "premium_user_plan" {
  namespace_key = flipt_namespace.production.key
  segment_key   = flipt_segment.premium_users.key
  type          = "STRING_COMPARISON_TYPE"
  property      = "plan"
  operator      = "eq"
  value         = "premium"
  description   = "Match premium plan users"
}

# Create a flag with metadata
resource "flipt_flag" "experimental_features" {
  namespace_key = flipt_namespace.production.key
  key           = "experimental-features"
  name          = "Experimental Features"
  description   = "Toggle experimental features"
  enabled       = true
  type          = "VARIANT_FLAG_TYPE"
  metadata = {
    team  = "platform"
    stage = "alpha"
  }
}

# Create variants for experimental features
resource "flipt_variant" "experimental_on" {
  namespace_key = flipt_namespace.production.key
  flag_key      = flipt_flag.experimental_features.key
  key           = "on"
  name          = "Features On"
  description   = "All experimental features enabled"
}

resource "flipt_variant" "experimental_off" {
  namespace_key = flipt_namespace.production.key
  flag_key      = flipt_flag.experimental_features.key
  key           = "off"
  name          = "Features Off"
  description   = "All experimental features disabled"
}

# Create a rule with multiple segments (OR operator)
resource "flipt_rule" "experimental_rule" {
  namespace_key    = flipt_namespace.production.key
  flag_key         = flipt_flag.experimental_features.key
  segment_keys     = [flipt_segment.beta_users.key, flipt_segment.premium_users.key]
  segment_operator = "OR_SEGMENT_OPERATOR"
  rank             = 1
}

# ============================================
# OUTPUTS - Summary of created resources
# ============================================

output "namespace_key" {
  value       = flipt_namespace.production.key
  description = "Production namespace key"
}

output "variant_flag_key" {
  value       = flipt_flag.new_ui.key
  description = "Variant flag key"
}

output "boolean_flag_key" {
  value       = flipt_flag.maintenance_mode.key
  description = "Boolean flag key"
}

output "segment_keys" {
  value       = [flipt_segment.beta_users.key, flipt_segment.premium_users.key]
  description = "Created segment keys"
}

output "rule_count" {
  value       = 2
  description = "Number of rules created"
}
