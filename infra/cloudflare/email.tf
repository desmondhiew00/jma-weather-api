# Contact address for the site: contact@tenkinow.com forwards to a personal
# inbox. Cloudflare Email Routing is free and receive-only, which is all a
# published contact point for licence, takedown and security mail needs.
#
# Two steps stay in the dashboard, once, before the first apply:
#
#  1. Enable Email Routing for the zone (its wizard writes the MX and SPF
#     records). cloudflare_email_routing_settings and _dns would do this, but
#     provider 5.23.0+ cannot decode that resource (a schema/model mismatch
#     on support_subaddress, cloudflare/terraform-provider-cloudflare #7301).
#     Enabling is a one-time toggle; the rules below are the part that
#     changes, so Terraform owns those and not the switch.
#  2. Add the destination address and click the link Cloudflare mails it.
#     Forwarding stays inert until it is verified, and no API can click it.

# A literal matcher, deliberately not a catch-all: a catch-all turns the domain
# into a spam funnel, and every address worth answering is listed here.
resource "cloudflare_email_routing_rule" "contact" {
  zone_id = var.cloudflare_zone_id
  name    = "contact@ -> ${var.contact_forward_to}"
  enabled = true

  matchers = [{
    type  = "literal"
    field = "to"
    value = "contact@${var.root_domain}"
  }]

  actions = [{
    type  = "forward"
    value = [var.contact_forward_to]
  }]

}

# security.txt names this one, so it has to resolve somewhere.
resource "cloudflare_email_routing_rule" "security" {
  zone_id = var.cloudflare_zone_id
  name    = "security@ -> ${var.contact_forward_to}"
  enabled = true

  matchers = [{
    type  = "literal"
    field = "to"
    value = "security@${var.root_domain}"
  }]

  actions = [{
    type  = "forward"
    value = [var.contact_forward_to]
  }]

}
