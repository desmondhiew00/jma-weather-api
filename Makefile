SHELL := /bin/bash
REGISTRY := ghcr.io/desmondhiew00
TAG ?= $(shell git rev-parse --short HEAD)
# SSH target. api.tenkinow.com is a PROXIED Cloudflare record, so it resolves to
# Cloudflare's anycast IPs and only carries HTTP/HTTPS. SSH to that name cannot
# work. Grey-clouding a hostname would republish the origin IP and undo the
# firewall, so the target is the IP, read from Terraform.
#
# Override with `make deploy SERVER=tenkinow` if you keep an ~/.ssh/config entry.
# The Terraform root holding the server and its DNS record. Override it to
# deploy to a second origin standing alongside the first.
TF_DIR ?= infra/hetzner
API_HOSTNAME ?= $(shell terraform -chdir=$(TF_DIR) output -raw api_hostname 2>/dev/null)
SERVER ?= root@$(shell terraform -chdir=$(TF_DIR) output -raw server_ipv4 2>/dev/null)

# The key is named explicitly rather than relying on ~/.ssh/config, because
# SERVER is an IP and a `Host` alias entry would not match it.
SSH_KEY ?= ~/.ssh/tenkinow_ed25519
SSH := ssh -i $(SSH_KEY) -o IdentitiesOnly=yes
REMOTE_DIR ?= /srv/tenkinow
PAGES_PROJECT ?= tenkinow-web

# Cloudflare credentials default to the Terraform tfvars, which is gitignored
# and already on any machine that can run terraform here. Overridable from the
# environment, so CI can supply them as secrets instead.
CF_TFVARS ?= infra/cloudflare/terraform.tfvars
tfvar = $(shell sed -n 's/^$(1) *= *"\(.*\)"/\1/p' $(CF_TFVARS) 2>/dev/null)
CLOUDFLARE_API_TOKEN ?= $(call tfvar,cloudflare_api_token)
CLOUDFLARE_ACCOUNT_ID ?= $(call tfvar,cloudflare_account_id)
CLOUDFLARE_ZONE_ID ?= $(call tfvar,cloudflare_zone_id)
export CLOUDFLARE_API_TOKEN
export CLOUDFLARE_ACCOUNT_ID
LOCAL_DSN ?= postgres://weather:weather@localhost:5433/weather?sslmode=disable

export DATABASE_URL ?= $(LOCAL_DSN)

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-18s\033[0m %s\n",$$1,$$2}'

## --- local development ---

.PHONY: up
up: ## Start the local Postgres
	docker compose up -d

.PHONY: down
down: ## Stop the local Postgres
	docker compose down

.PHONY: run-api
run-api: ## Run the API on the host
	go run ./cmd/api

.PHONY: run-worker
run-worker: ## Run the worker on the host (applies migrations)
	go run ./cmd/worker

.PHONY: sqlc
sqlc: ## Regenerate the sqlc query code
	go tool sqlc generate

.PHONY: sqlc-check
sqlc-check: ## Fail if generated code is stale relative to query.sql
	go tool sqlc diff

.PHONY: migrate-up
migrate-up: ## Apply migrations (the worker also does this at startup)
	go run ./cmd/migrate up

.PHONY: migrate-down
migrate-down: ## Roll back the last migration
	go run ./cmd/migrate down

## --- quality ---

.PHONY: test
test: ## Run all tests (repository tests need Docker)
	go test ./... -timeout 600s

.PHONY: test-short
test-short: ## Run tests that need no database
	go test ./... -short

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run

.PHONY: check
check: sqlc-check lint test ## Everything CI runs

## --- images and deploy ---

.PHONY: docker-build
docker-build: ## Build both images locally for linux/amd64
	docker build --platform linux/amd64 --build-arg GIT_SHA=$(TAG) \
		-f Dockerfile.api   -t $(REGISTRY)/jma-weather-api-api:$(TAG) .
	docker build --platform linux/amd64 --build-arg GIT_SHA=$(TAG) \
		-f Dockerfile.worker -t $(REGISTRY)/jma-weather-api-worker:$(TAG) .

.PHONY: deploy
deploy: ## Deploy TAG (default: current commit) to the server
	@test "$(SERVER)" != "root@" || { \
		echo "SERVER is empty: terraform output unavailable."; \
		echo "Run from a machine with terraform state access to $(TF_DIR),"; \
		echo "or pass SERVER=root@<ip> (and API_HOSTNAME=<host>) explicitly."; \
		exit 1; }
	@echo "deploying $(TAG) to $(SERVER)"
	# --inplace is required: compose bind-mounts Caddyfile as a single file,
	# and rsync's default write-temp-then-rename changes the inode, leaving
	# the running container mounted on the old one.
	rsync -av --inplace -e "$(SSH)" deploy/compose.yaml deploy/Caddyfile deploy/backup.sh deploy/metrics.sh $(SERVER):$(REMOTE_DIR)/
	# TAG is written into .env, not just passed for this one command: compose
	# reads .env on every invocation, so a bare `docker compose up -d` on the
	# box would otherwise roll back to whatever tag was last written there.
	$(SSH) $(SERVER) 'cd $(REMOTE_DIR) \
		&& chmod +x backup.sh metrics.sh \
		&& sed -i "s/^TAG=.*/TAG=$(TAG)/" .env \
		&& grep -q "^TAG=" .env || echo "TAG=$(TAG)" >> .env \
		&& docker compose pull && docker compose up -d'
	@echo "deployed; verifying"
	@sleep 5
	@test -n "$(API_HOSTNAME)" || echo "WARNING: API_HOSTNAME unknown, skipping verification"
	@test -z "$(API_HOSTNAME)" || curl -fsS https://$(API_HOSTNAME)/healthz | tee /dev/stderr | grep -q '"sha":"$(TAG)"' \
		&& echo "OK: $(TAG) is live" \
		|| echo "WARNING: /healthz did not report $(TAG)"

.PHONY: deploy-web
deploy-web: ## Build web/ and deploy it to Cloudflare Pages
	@test -n "$(CLOUDFLARE_API_TOKEN)" || { \
		echo "No Cloudflare token: $(CF_TFVARS) is missing and the environment"; \
		echo "does not set CLOUDFLARE_API_TOKEN. The token needs Account >"; \
		echo "Cloudflare Pages > Edit."; \
		exit 1; }
	pnpm --dir web install --frozen-lockfile
	pnpm --dir web run build
	# Pages serves the static build from the edge worldwide. Deliberately not
	# Caddy on the origin: Cloudflare does not cache HTML by default, so every
	# page load would cross to Singapore for ~480ms.
	# Run wrangler from the repo root, not web/. npx inside the pnpm workspace
	# installs wrangler as a devDependency and rewrites pnpm-lock.yaml; the Go
	# root has no package.json, so there is nothing there for it to pollute.
	npx --yes wrangler@latest pages deploy web/dist \
		--project-name=$(PAGES_PROJECT) --branch=main

.PHONY: purge
purge: ## Purge the Cloudflare cache (run after a deploy that changes a response)
	@test -n "$(CLOUDFLARE_ZONE_ID)" || { echo "CLOUDFLARE_ZONE_ID is unset"; exit 1; }
	@curl -fsS -X POST \
		"https://api.cloudflare.com/client/v4/zones/$(CLOUDFLARE_ZONE_ID)/purge_cache" \
		-H "Authorization: Bearer $(CLOUDFLARE_API_TOKEN)" \
		-H "Content-Type: application/json" \
		--data '{"purge_everything":true}' \
		| grep -q '"success":true' && echo "cache purged" || { echo "purge failed"; exit 1; }

.PHONY: logs
logs: ## Tail the server logs
	$(SSH) $(SERVER) 'cd $(REMOTE_DIR) && docker compose logs -f --tail=100'

.PHONY: db-tunnel
db-tunnel: ## Forward the server Postgres to localhost:5434
	@echo "psql postgres://weather:\$$POSTGRES_PASSWORD@localhost:5434/weather?sslmode=disable"
	$(SSH) -N -L 5434:127.0.0.1:5432 $(SERVER)
