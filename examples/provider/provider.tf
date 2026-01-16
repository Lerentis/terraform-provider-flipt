# Copyright (c) terraform-provider-flipt contributors

# Basic provider configuration without authentication
provider "flipt" {
  endpoint = "http://localhost:8080"
}

# Provider configuration with static token authentication
# provider "flipt" {
#   endpoint = "http://localhost:8080"
#   token    = "your_static_token_here"  # Uses Bearer authentication
# }

# Provider configuration with JWT authentication
# provider "flipt" {
#   endpoint = "http://localhost:8080"
#   jwt      = "your_jwt_token_here"     # Uses JWT authentication
# }
