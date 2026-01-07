data "flipt_flag" "example" {
  namespace_key = "production"
  key           = "new-feature"
}

output "flag_name" {
  value = data.flipt_flag.example.name
}

output "flag_enabled" {
  value = data.flipt_flag.example.enabled
}

output "flag_type" {
  value = data.flipt_flag.example.type
}
