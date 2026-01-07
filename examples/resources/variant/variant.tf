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

resource "flipt_variant" "example" {
  namespace_key = "default"
  flag_key      = "my-feature"
  key           = "variant-a"
  name          = "Variant A"
  description   = "First variant option"
  attachment    = jsonencode({ color = "red", size = "large" })
}

# Import existing variant
# terraform import flipt_variant.example variant-id
