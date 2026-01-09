data "flipt_environment" "default" {
  key = "default"
}

output "environment_name" {
  value = data.flipt_environment.default.name
}

output "is_default" {
  value = data.flipt_environment.default.default
}
