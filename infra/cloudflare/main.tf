# This root owns the Cloudflare zone configuration only: the cache and
# rate-limit rulesets, the coordinate-rounding Worker and the backup bucket.
# All of it is zone-wide and matches on path, so it applies to whichever origin
# serves the hostname. The compute and its DNS record live in infra/hetzner.

locals {
  # "api.tenkinow.com" -> "tenkinow.com", for the Worker route wildcard.
  zone_domain = join(".", slice(split(".", var.api_hostname), 1, length(split(".", var.api_hostname))))
}

# Cloudflare does not cache JSON by default, so without this rule every request
# reaches the origin in Tokyo. This rule keeps egress within budget.
resource "cloudflare_ruleset" "cache" {
  zone_id = var.cloudflare_zone_id
  name    = "tenkinow cache"
  kind    = "zone"
  phase   = "http_request_cache_settings"

  rules = [
    {
      description = "Cache latest observations at the edge"
      # /now is the all-stations document: one response, identical for every
      # caller, regenerated every ten minutes like /latest. Left out of this
      # rule it was DYNAMIC on every request and paid a full origin round trip.
      expression = "(starts_with(http.request.uri.path, \"/v1/weather/latest\") or starts_with(http.request.uri.path, \"/v1/weather/now\"))"
      action     = "set_cache_settings"
      action_parameters = {
        cache = true
        # Observations only change every ten minutes, and are already ~6 minutes
        # old when JMA publishes them, so a 60s TTL was ten times stricter than
        # the data warrants. A cache HIT answers in ~15ms; a miss pays three
        # round trips to the origin, because Cloudflare opens a fresh TLS
        # connection to an idle origin every time (measured: never resumed).
        edge_ttl = {
          mode    = "override_origin"
          default = 300
        }
        # Cloudflare's Free-plan Browser Cache TTL defaults to 4 hours and
        # overrides the handler's max-age=60, which would let a browser serve
        # four-hour-old "latest" weather.
        browser_ttl = {
          mode = "respect_origin"
        }
      }
    },
    {
      description = "Cache historical queries hard: observations are immutable once made"
      expression  = "(starts_with(http.request.uri.path, \"/v1/weather/history\") or starts_with(http.request.uri.path, \"/v1/weather/at\"))"
      action      = "set_cache_settings"
      action_parameters = {
        cache = true
        edge_ttl = {
          mode    = "override_origin"
          default = 3600
        }
        browser_ttl = {
          mode = "respect_origin"
        }
      }
    },
  ]
}

# The single rate-limiting rule the Free plan allows. On Free the counting
# period and mitigation timeout are both fixed at 10s and counting is IP-based
# only. That is enough to stop a looping client, the realistic abuse here.
resource "cloudflare_ruleset" "rate_limit" {
  zone_id = var.cloudflare_zone_id
  name    = "tenkinow rate limit"
  kind    = "zone"
  phase   = "http_ratelimit"

  rules = [
    {
      description = "Throttle a single hot client"
      expression  = "(starts_with(http.request.uri.path, \"/v1/weather/\"))"
      action      = "block"
      ratelimit = {
        characteristics     = ["ip.src", "cf.colo.id"]
        period              = 10
        requests_per_period = 50
        mitigation_timeout  = 10
      }
    },
  ]
}

# Rounds lat/lon before the cache lookup; see coord-round.js for why. This is
# a Worker rather than a Transform Rule because regex_replace() (the only way
# to do arithmetic-ish rewriting in rules) is Business plan and above.
resource "cloudflare_workers_script" "coord_round" {
  account_id  = var.cloudflare_account_id
  script_name = "tenkinow-coord-round"
  content     = file("${path.module}/coord-round.js")
}

# Wildcard rather than a single hostname, so a second origin on another
# subdomain (the Tokyo box in infra/aws) is covered by the same script without
# the two states having to share a resource.
resource "cloudflare_workers_route" "coord_round" {
  zone_id = var.cloudflare_zone_id
  pattern = "*.${local.zone_domain}/v1/weather/*"
  script  = cloudflare_workers_script.coord_round.script_name
}

# www redirects to the apex rather than serving alongside it. astro.config.mjs
# sets site: https://tenkinow.com, so canonical tags and the sitemap already
# point there; serving both would give every page two URLs and split indexing.
#
# This also has to exist for www to work at all: the record flattens onto the
# Pages project, which only answers for hostnames registered as its custom
# domains, so without a redirect www returns 522. Redirect rules run before
# that routing, so the 301 is issued at the edge and never reaches Pages.
resource "cloudflare_ruleset" "redirect" {
  zone_id = var.cloudflare_zone_id
  name    = "tenkinow redirects"
  kind    = "zone"
  phase   = "http_request_dynamic_redirect"

  rules = [
    {
      description = "Send www to the apex"
      expression  = "(http.host eq \"www.${local.zone_domain}\")"
      action      = "redirect"
      action_parameters = {
        from_value = {
          status_code           = 301
          preserve_query_string = true
          target_url = {
            expression = "concat(\"https://${local.zone_domain}\", http.request.uri.path)"
          }
        }
      }
    },
  ]
}

resource "cloudflare_r2_bucket" "backups" {
  account_id = var.cloudflare_account_id
  name       = var.r2_bucket
  location   = "apac"
}
