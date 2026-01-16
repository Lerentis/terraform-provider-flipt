# Example: Using static token authentication
provider "flipt" {
  endpoint = "http://localhost:8080"
  token    = "your_static_token_here"
}

# Example: Using JWT authentication (commented out - use only one auth method)
# provider "flipt" {
#   endpoint = "http://localhost:8080"
#   jwt      = "your_jwt_token_here"
# }

# Example: No authentication (for testing or unauthenticated instances)
# provider "flipt" {
#   endpoint = "http://localhost:8080"
# }

# Example resource using authenticated provider
resource "flipt_namespace" "example" {
  environment_key = "default"
  key             = "example-namespace"
  name            = "Example Namespace"
  description     = "This namespace is created with authentication"
}
