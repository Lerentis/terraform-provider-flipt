# Flipt Terraform Provider

Terraform provider for managing [Flipt](https://www.flipt.io/) feature flags.

## Resources

The provider includes the following resources:

### Core Resources

- **`flipt_namespace`** - Manage Flipt namespaces for organizing feature flags
- **`flipt_flag`** - Manage feature flags (both variant and boolean types)
- **`flipt_segment`** - Manage user segments for targeting
- **`flipt_variant`** - Manage flag variants (belongs to a flag)
- **`flipt_constraint`** - Manage segment constraints (belongs to a segment)
- **`flipt_rule`** - Manage evaluation rules (links flags to segments)
- **`flipt_distribution`** - Manage variant distributions (belongs to a rule)

## Usage

```hcl
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
  name        = "Production"
  description = "Production environment flags"
}

# Create a feature flag
resource "flipt_flag" "new_feature" {
  namespace_key = flipt_namespace.production.key
  key           = "new-feature"
  name          = "New Feature"
  description   = "Enable new feature"
  enabled       = true
  type          = "VARIANT_FLAG_TYPE"
}

# Create a variant
resource "flipt_variant" "variant_a" {
  namespace_key = flipt_namespace.production.key
  flag_key      = flipt_flag.new_feature.key
  key           = "variant-a"
  name          = "Variant A"
}

# Create a segment
resource "flipt_segment" "beta_users" {
  namespace_key = flipt_namespace.production.key
  key           = "beta-users"
  name          = "Beta Users"
  match_type    = "ALL_MATCH_TYPE"
}

# Add a constraint to the segment
resource "flipt_constraint" "email_constraint" {
  namespace_key = flipt_namespace.production.key
  segment_key   = flipt_segment.beta_users.key
  type          = "STRING_COMPARISON_TYPE"
  property      = "email"
  operator      = "suffix"
  value         = "@beta.example.com"
}

# Create a rule
resource "flipt_rule" "beta_rule" {
  namespace_key = flipt_namespace.production.key
  flag_key      = flipt_flag.new_feature.key
  segment_key   = flipt_segment.beta_users.key
  rank          = 1
}

# Create a distribution
resource "flipt_distribution" "variant_a_dist" {
  namespace_key = flipt_namespace.production.key
  flag_key      = flipt_flag.new_feature.key
  rule_id       = flipt_rule.beta_rule.id
  variant_id    = flipt_variant.variant_a.id
  rollout       = 100.0
}
```

## Resource Hierarchy

```
Namespace
├── Flag
│   ├── Variant (multiple)
│   ├── Rule (multiple)
│   │   └── Distribution (multiple)
│   └── Rollout (multiple)
└── Segment
    └── Constraint (multiple)
```

## Examples

See the [examples](./examples) directory for more detailed usage examples:

- [Complete Example](./examples/complete/main.tf) - Full feature flag setup with all resources
- [Namespace](./examples/resources/namespace/namespace.tf)
- [Flag](./examples/resources/flag/flag.tf)
- [Segment](./examples/resources/segment/segment.tf)
- [Variant](./examples/resources/variant/variant.tf)
- [Constraint](./examples/resources/constraint/constraint.tf)
- [Rule](./examples/resources/rule/rule.tf)
- [Distribution](./examples/resources/distribution/distribution.tf)

## Building

```bash
go build
```

## Development

This provider uses the [Flipt REST SDK](https://github.com/Lerentis/flipt-server-rest-sdk-go) for API communication.

## License

MIT
