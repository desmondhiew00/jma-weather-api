#!/usr/bin/env bash
# Traffic figures from Caddy's access log.
#
# Caddy writes one JSON line per request to logs/access.log and rolls it, so
# every count here comes from files that already exist: nothing is written to
# Postgres and nothing runs between requests.
#
# Run on the server, from the directory holding logs/ (/srv/tenkinow).
#
#   ./metrics.sh endpoints          requests per endpoint
#   ./metrics.sh endpoints 7        the same, last 7 days only
#
# Two caveats before quoting a number:
#   - These are ORIGIN hits. A response served from Cloudflare's cache never
#     reaches Caddy, so real API traffic is higher than what is counted here.
#   - History reaches back only as far as the oldest kept log file: 90 days or
#     50 rolls, whichever ends first (see the log block in the Caddyfile).
set -euo pipefail

LOG_GLOB=${LOG_GLOB:-logs/access.log*}

usage() {
	cat <<'USAGE'
Traffic figures from Caddy's access log. Run on the server, from the directory
holding logs/ (/srv/tenkinow). Times are JST.

Two caveats worth remembering before quoting a number:
  - These are ORIGIN hits. A response served from Cloudflare's cache never
    reaches Caddy, so real API traffic is higher than what is counted here.
  - History reaches back only as far as the oldest kept log file: 90 days or
    50 rolls, whichever ends first (see the log block in the Caddyfile).

Commands:
  endpoints        requests per endpoint, busiest first
  hourly           requests per hour per endpoint
  daily            requests per day, all endpoints
  monthly          requests per month, all endpoints
  status           status code per endpoint
  errors           the last 20 responses of 500 and up
  latency          request count, mean and p95 per endpoint
  ips              top client IPs (the real caller, not Cloudflare's edge)
  countries        top countries, from Cloudflare's Cf-Ipcountry header
  agents           top user agents
  bytes            bytes served per endpoint
  params           most requested query strings on /v1/weather/latest
  hours            busiest hour of the day, summed over the window

Every command takes an optional number of days to look back:
  ./metrics.sh status 30
USAGE
}

# read emits one access-log JSON object per line, dropping Caddy's non-access
# lines (startup, TLS, errors), and anything older than the requested window.
read_log() {
	local since=0
	[[ -n ${1:-} ]] && since=$(date -d "$1 days ago" +%s)

	# -f lets one zcat read both the current log and the gzipped rolls.
	zcat -f $LOG_GLOB |
		jq -rR --argjson since "$since" \
			'fromjson? | select(.msg == "handled request") | select(.ts > $since)'
}

# path strips the query string: /v1/weather/latest?lat=35&lon=139 and its
# thousand sibling coordinates are one endpoint, not a thousand.
path='.request.uri | split("?")[0]'

# JST, because that is the day the traffic happened in.
jst='.ts + 32400'

cmd=${1:-help}
days=${2:-}

case "$cmd" in
endpoints)
	read_log "$days" | jq -r "$path" | sort | uniq -c | sort -rn
	;;
hourly)
	read_log "$days" | jq -r "(($jst)|strftime(\"%Y-%m-%dT%H\")) + \" \" + ($path)" |
		sort | uniq -c | sort -k2
	;;
daily)
	read_log "$days" | jq -r "($jst)|strftime(\"%Y-%m-%d\")" | sort | uniq -c
	;;
monthly)
	read_log "$days" | jq -r "($jst)|strftime(\"%Y-%m\")" | sort | uniq -c
	;;
status)
	read_log "$days" | jq -r "[($path), (.status|tostring)] | join(\" \")" |
		sort | uniq -c | sort -rn
	;;
errors)
	read_log "$days" | jq -r "select(.status >= 500) |
		(($jst)|strftime(\"%m-%d %H:%M\")) + \" \" + (.status|tostring) + \" \" + .request.uri" |
		tail -20
	;;
latency)
	# Sorted per endpoint so the p95 is a real order statistic rather than a
	# running estimate; the volumes here are far too small for that to matter.
	read_log "$days" | jq -r "[($path), (.duration|tostring)] | @tsv" |
		sort -k1,1 -k2,2g |
		awk -F'\t' '
			{ n[$1]++; v[$1 SUBSEP n[$1]] = $2; sum[$1] += $2 }
			END {
				for (e in n) {
					i = int(n[e] * 0.95); if (i < 1) i = 1
					printf "%-30s n=%-7d mean=%6.0fms p95=%6.0fms\n", e, n[e], sum[e] / n[e] * 1000, v[e SUBSEP i] * 1000
				}
			}' | sort -k2 -t= -rn
	;;
ips)
	# Cf-Connecting-Ip is the caller. remote_ip is whichever Cloudflare edge
	# machine relayed the request, which is never interesting.
	read_log "$days" | jq -r '.request.headers["Cf-Connecting-Ip"][0] // .request.client_ip' |
		sort | uniq -c | sort -rn | head -20
	;;
countries)
	read_log "$days" | jq -r '.request.headers["Cf-Ipcountry"][0] // "??"' |
		sort | uniq -c | sort -rn | head -20
	;;
agents)
	read_log "$days" | jq -r '.request.headers["User-Agent"][0] // "-"' |
		sort | uniq -c | sort -rn | head -20
	;;
bytes)
	read_log "$days" | jq -r "[($path), (.size|tostring)] | @tsv" |
		awk -F'\t' '{ s[$1] += $2 } END { for (e in s) printf "%-30s %8.1f MB\n", e, s[e] / 1048576 }' |
		sort -k2 -rn
	;;
params)
	read_log "$days" | jq -r 'select(.request.uri | startswith("/v1/weather/latest")) | .request.uri' |
		sed 's/[^?]*?*//' | sort | uniq -c | sort -rn | head -20
	;;
hours)
	read_log "$days" | jq -r "($jst)|strftime(\"%H\")" | sort | uniq -c | sort -k2
	;;
help | -h | --help)
	usage
	;;
*)
	echo "unknown command: $cmd" >&2
	usage >&2
	exit 1
	;;
esac
