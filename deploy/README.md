# Deploying

One Hetzner box in Singapore runs Docker Compose behind Cloudflare.

## First time

1. Create the Terraform state bucket by hand. Terraform can't create its own state bucket:
   `gcloud storage buckets create gs://tenkinow-tfstate --location=asia-northeast1`
2. Point `tenkinow.com`'s nameservers at Cloudflare. The registrar stays Namecheap.
3. Copy both tfvars files and fill them in:

   ```console
   $ cp infra/cloudflare/terraform.tfvars.example infra/cloudflare/terraform.tfvars  # zone config
   $ cp infra/hetzner/terraform.tfvars.example infra/hetzner/terraform.tfvars        # server + DNS record
   ```

   **SSH key.** `ssh_public_key` takes the full one-line contents of a `.pub` file. Use a dedicated key, so revoking server access doesn't touch anything else:

   ```console
   $ ssh-keygen -t ed25519 -C "tenkinow-server" -f ~/.ssh/tenkinow_ed25519
   $ cat ~/.ssh/tenkinow_ed25519.pub
   ```

   **Cloudflare token.** My Profile → API Tokens → Create Custom Token (not the Global API Key), with these permissions:

   | Permission | Scope |
   | --- | --- |
   | Zone → DNS → Edit | Zone |
   | Zone → SSL and Certificates → Edit | Zone |
   | Zone → Cache Rules → Edit | Zone |
   | Zone → Zone WAF → Edit | Zone |
   | Account → Workers R2 Storage → Edit | Account |

   Zone Resources: Include → Specific zone → your domain. Account Resources: Include → your account.

   **Zone and account IDs.** This call returns both and also checks the token works:

   ```console
   $ curl -s -H "Authorization: Bearer $CF_TOKEN" \
       'https://api.cloudflare.com/client/v4/zones?name=tenkinow.com' \
       | jq -r '.result[] | "zone_id=\(.id)\naccount_id=\(.account.id)"'
   ```

   The dashboard shows them too: the Zone ID is in the API box on the zone Overview page, and the Account ID is the first path segment of the dashboard URL.

   Do these first or the apply fails:
   - Activate R2 once by hand (R2 → Get started, accept the billing terms). The API won't provision it.
   - Enable API Access for your user on the account, or Origin CA issuance errors out.
   - Make sure the zone's cache and rate-limit rulesets are empty. Terraform takes over both and replaces any rules made in the dashboard.
4. Apply both roots. They have separate state, so you can rebuild the server or move it to another provider without touching the zone's cache, rate-limit and Worker config:

   ```console
   $ terraform -chdir=infra/cloudflare init && terraform -chdir=infra/cloudflare apply  # zone
   $ terraform -chdir=infra/hetzner init && terraform -chdir=infra/hetzner apply        # server
   ```
5. Set Cloudflare SSL mode to Full (strict).
6. Add an SSH entry for the box. `api.tenkinow.com` is a proxied Cloudflare record that only carries HTTP/HTTPS, so SSH has to use the server's IP:

   ```
   Host tenkinow
     HostName <terraform -chdir=infra/hetzner output -raw server_ipv4>
     User root
     IdentityFile ~/.ssh/tenkinow_ed25519
     IdentitiesOnly yes
   ```

   The Makefile reads the IP from Terraform output on its own. With this entry you can also run `make deploy SERVER=tenkinow`.
7. On the box, create `/srv/tenkinow/` containing:
   - `.env`, copied from `.env.example`, with `chmod 600`
   - `origin.pem` and `origin-key.pem`, from `terraform -chdir=infra/hetzner output -raw origin_certificate` and `origin_private_key`

   Set `API_HOSTNAME` in `.env` to the hostname this box serves. Caddy uses it as the site address.
8. Push to `main` once so CI publishes both images. Then make each package public: GitHub → Packages → `jma-weather-api-api` → Package settings → Change visibility → Public. Repeat for `jma-weather-api-worker`.

   GHCR packages start private even when the repo is public. Until you flip them, `docker compose pull` on the server fails with a 401. Public packages need no credentials on the box. To keep them private instead, give the server a PAT with `read:packages` and run `docker login ghcr.io`.
9. `make deploy`
10. Create a GCP uptime check on `https://api.tenkinow.com/healthz` at a 1-minute interval. Attach a verified email notification channel. Without a channel, nobody gets notified.
11. Fill in `R2_ACCOUNT_ID`, `R2_ACCESS_KEY_ID` and `R2_SECRET_ACCESS_KEY` in `/srv/tenkinow/.env`. They come from R2 → API → Manage API tokens, and they're S3 access keys, not the Cloudflare API token. Then run a backup and restore it into a scratch database to prove it works:

    ```console
    $ ssh tenkinow
    # /srv/tenkinow/backup.sh
    # cd /srv/tenkinow
    # LATEST=$(ls -t /var/lib/tenkinow/backups/*.sql.gz | head -1)
    # docker compose exec -T postgres psql -U weather -d postgres -c 'CREATE DATABASE restore_test;' </dev/null
    # gunzip -c "$LATEST" | docker compose exec -T postgres psql -U weather -d restore_test -v ON_ERROR_STOP=1
    # docker compose exec -T postgres psql -U weather -d restore_test -c 'SELECT count(*) FROM observations;' </dev/null
    # docker compose exec -T postgres psql -U weather -d postgres -c 'DROP DATABASE restore_test;' </dev/null
    ```

    `docker compose exec -T` reads stdin. In a script, add `</dev/null` or it eats the rest of the script.

## Releases

CI builds both images on every push to `main` and tags them with the commit SHA and `latest`. Deploys are manual:

```console
make deploy              # newest green main
make deploy TAG=abc1234  # a specific commit, e.g. to roll back
make logs
```

Both images carry the same SHA, and `/healthz` reports the SHA the box is running.

## Operations

- **Cost:** about €16/month (CPX12 €15.49 + €0.50 for IPv4). Cloudflare, GCP monitoring and R2 are on free tiers.
- **Traffic:** Singapore includes only 0.5 TB, then €7.40/TB (the EU gets 20 TB and €1.00/TB). Cloudflare doesn't cache JSON by default, so the cache rules in Terraform keep egress under the limit. Don't remove them.
- **Firewall:** the origin only accepts ports 80/443 from Cloudflare's IP ranges, which Terraform reads at plan time. To reach the origin directly, add your IP to the firewall in Hetzner's console.
- **Backups** are the only real recovery path. JMA keeps about three days of per-station history and we keep 30, so anything lost beyond those three days can't be re-fetched.
- **Memory:** the box has 1 vCPU and 2 GB of RAM for four containers. Postgres is tuned down (`shared_buffers=512MB`, `max_connections=25`), with 2 GB of swap in case of OOM. Don't add anything else to it.
- **Logs** use Docker's `json-file` driver with rotation. Without rotation Docker logs grow until the disk fills and Postgres goes down.
