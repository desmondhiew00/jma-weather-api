terraform {
  required_version = ">= 1.9"

  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5.0"
    }
  }

  # GCS rather than a local file: native locking and versioning, and it stays
  # put regardless of where the compute lives. Create the bucket by hand once;
  # Terraform cannot bootstrap its own state.
  backend "gcs" {
    bucket = "tenkinow-tfstate"
    prefix = "jma-weather-api"
  }
}

provider "cloudflare" {
  api_token = var.cloudflare_api_token
}
