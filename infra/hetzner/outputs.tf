output "server_ipv4" {
  description = "Origin IP. Stops being public knowledge once the record is proxied."
  value       = hcloud_server.api.ipv4_address
}

output "ssh" {
  value = "ssh root@${hcloud_server.api.ipv4_address}"
}

output "api_hostname" {
  value = var.api_hostname
}

# Write these to /srv/tenkinow/origin.pem and origin-key.pem on the server.
output "origin_certificate" {
  description = "Cloudflare Origin CA certificate (PEM)"
  value       = cloudflare_origin_ca_certificate.api.certificate
  sensitive   = true
}

output "origin_private_key" {
  description = "Origin certificate private key (PEM)"
  value       = tls_private_key.origin.private_key_pem
  sensitive   = true
}
