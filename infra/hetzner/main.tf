# Cloudflare's published ranges, read at plan time rather than pasted into a
# variable, so the allowlist cannot silently drift as Cloudflare changes it.
data "http" "cloudflare_ips_v4" {
  url = "https://www.cloudflare.com/ips-v4"
}

data "http" "cloudflare_ips_v6" {
  url = "https://www.cloudflare.com/ips-v6"
}

locals {
  # "api.tenkinow.com" -> "tenkinow.com", for the Worker route wildcard.
  zone_domain = join(".", slice(split(".", var.api_hostname), 1, length(split(".", var.api_hostname))))

  cloudflare_ips = concat(
    compact(split("\n", trimspace(data.http.cloudflare_ips_v4.response_body))),
    compact(split("\n", trimspace(data.http.cloudflare_ips_v6.response_body))),
  )
}

resource "hcloud_ssh_key" "admin" {
  name       = "tenkinow-admin"
  public_key = var.ssh_public_key
}

resource "hcloud_firewall" "api" {
  name = "tenkinow-api"

  # 80/443 only from Cloudflare. Without this the origin IP stays directly
  # reachable, which defeats both the proxy's IP hiding and the cache's job of
  # capping egress. The IP is exposed during zone activation anyway.
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "80"
    source_ips = local.cloudflare_ips
  }

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "443"
    source_ips = local.cloudflare_ips
  }

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "22"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  rule {
    direction  = "in"
    protocol   = "icmp"
    source_ips = ["0.0.0.0/0", "::/0"]
  }
}

resource "hcloud_server" "api" {
  name         = "tenkinow"
  server_type  = var.server_type
  location     = var.location
  image        = "debian-13"
  ssh_keys     = [hcloud_ssh_key.admin.id]
  firewall_ids = [hcloud_firewall.api.id]
  user_data    = file("${path.module}/provision.sh")

  lifecycle {
    # Replacing the box would destroy the Postgres bind mount with it.
    prevent_destroy = true

    # A declared public_net block stores ipv4/ipv6 as 0 and then reads back as
    # a change that forces replacement. Both are enabled by default, so the
    # block is omitted and any drift here is ignored rather than acted on.
    ignore_changes = [public_net, user_data, image, ssh_keys]
  }
}

## --- Cloudflare ---

resource "cloudflare_dns_record" "api" {
  zone_id = var.cloudflare_zone_id
  name    = var.api_hostname
  type    = "A"
  content = hcloud_server.api.ipv4_address
  ttl     = 1 # "automatic"; required when proxied
  proxied = true
}

# The origin serves this instead of running ACME. Valid for 15 years, so there
# is no renewal machinery to forget about. Cloudflare SSL mode must be
# "Full (strict)" for it to be validated rather than merely accepted.
resource "tls_private_key" "origin" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_cert_request" "origin" {
  private_key_pem = tls_private_key.origin.private_key_pem

  subject {
    common_name = var.api_hostname
  }
}

resource "cloudflare_origin_ca_certificate" "api" {
  csr                = tls_cert_request.origin.cert_request_pem
  hostnames          = [var.api_hostname]
  request_type       = "origin-rsa"
  requested_validity = 5475
}
