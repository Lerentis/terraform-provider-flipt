data "flipt_segment" "example" {
  namespace_key = "production"
  key           = "beta-users"
}

output "segment_name" {
  value = data.flipt_segment.example.name
}

output "segment_match_type" {
  value = data.flipt_segment.example.match_type
}

output "segment_description" {
  value = data.flipt_segment.example.description
}
