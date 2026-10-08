variable "cloudflare_api_token" {
  description = "Cloudflare API token (Zone:Edit, DNS:Edit, SSL:Edit, R2:Edit)"
  type        = string
  sensitive   = true
}

variable "cloudflare_zone_id" {
  description = "Zone ID for the domain"
  type        = string
}

variable "cloudflare_account_id" {
  description = "Cloudflare account ID (for R2)"
  type        = string
}

variable "api_hostname" {
  description = "Hostname the API is served on"
  type        = string
  default     = "api.tenkinow.com"
}

variable "r2_bucket" {
  description = "R2 bucket for Postgres dumps"
  type        = string
  default     = "tenkinow-backups"
}

variable "root_domain" {
  description = "Apex domain, used for email routing addresses"
  type        = string
  default     = "tenkinow.com"
}

variable "contact_forward_to" {
  description = "Mailbox contact@ and security@ forward to. Must be verified in the Cloudflare dashboard first."
  type        = string
}
