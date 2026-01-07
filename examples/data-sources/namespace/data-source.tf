data "flipt_namespace" "example" {
  key = "production"
}

output "namespace_name" {
  value = data.flipt_namespace.example.name
}

output "namespace_description" {
  value = data.flipt_namespace.example.description
}
