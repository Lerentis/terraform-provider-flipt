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

resource "flipt_flag" "example" {
  namespace_key = "default"
  key           = "my-feature"
  name          = "My Feature Flag"
  description   = "Controls access to my feature"
  enabled       = true
  type          = "VARIANT_FLAG_TYPE"
}

# Import existing flag
# terraform import flipt_flag.example default/existing-flag-key
