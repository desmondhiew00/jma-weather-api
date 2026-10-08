output "api_hostname" {
  description = "Hostname the zone config applies to. Read by the Makefile to verify the right origin."
  value       = var.api_hostname
}
