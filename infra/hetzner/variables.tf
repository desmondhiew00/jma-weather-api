variable "hcloud_token" {
  description = "Hetzner Cloud API token"
  type        = string
  sensitive   = true
}

variable "cloudflare_api_token" {
  description = "Cloudflare API token (Zone:Edit, DNS:Edit, SSL:Edit)"
  type        = string
  sensitive   = true
}

variable "cloudflare_zone_id" {
  description = "Zone ID for the domain"
  type        = string
}

# Starts on a separate hostname so this box can be built and verified while the
# current origin still serves api.tenkinow.com. Switch to api.tenkinow.com at
# cutover, once this one answers correctly.
variable "api_hostname" {
  description = "Hostname this environment is served on"
  type        = string
  default     = "api-sg.tenkinow.com"
}

# Singapore is as close to Japan as Hetzner gets: ~87 ms RTT and ~480 ms for an
# uncached request, against ~270 ms from Tokyo. There is no Japanese region, so
# this is the floor for this provider.
variable "location" {
  description = "Hetzner location"
  type        = string
  default     = "sin"
}

variable "server_type" {
  description = "Hetzner server type. Singapore offers CPX/CCX only, all x86."
  type        = string
  default     = "cpx12"
}

variable "ssh_public_key" {
  description = "SSH public key allowed to log in"
  type        = string
}
