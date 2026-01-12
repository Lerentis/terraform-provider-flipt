terraform {
  required_providers {
    flipt = {
      source = "lerentis/flipt"
    }
  }
}

provider "flipt" {
  endpoint    = "http://localhost:8080"
  environment = "default"
}

data "flipt_variant" "example" {
  namespace_key = "default"
  flag_key      = "my-feature"
  key           = "variant-a"
}

output "variant_name" {
  value = data.flipt_variant.example.name
}

output "variant_attachment" {
  value = data.flipt_variant.example.attachment
}
