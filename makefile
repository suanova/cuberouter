WEB_DIR = ./web
API_DIR = .
DEV_WEB_PORT ?= 5173
DEV_COMPOSE_FILE = docker-compose.dev.yml
DEV_POSTGRES_SERVICE = postgres
DEV_API_SERVICE = new-api
DEV_POSTGRES_DB = new-api
DEV_POSTGRES_USER = root
DEV_SQLITE_PATH ?= one-api.db
CUBEROUTER_IMAGE_TAG ?= latest
OFFLINE_PACKAGE = cuberouter-$(CUBEROUTER_IMAGE_TAG).tar.gz

.PHONY: all build-web build-all-web start-api dev dev-api dev-api-rebuild dev-web reset-setup swag test offline-package

all: build-all-web start-api

build-web:
	@echo "Building web frontend..."
	@cd $(WEB_DIR) && bun install --frozen-lockfile
	@cd $(WEB_DIR) && DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$$(cat ../VERSION) bun run build

build-all-web: build-web

start-api:
	@echo "Starting api dev server..."
	@cd $(API_DIR) && go run . &

dev-api:
	@echo "Starting api services (docker)..."
	@docker compose -f $(DEV_COMPOSE_FILE) up -d

dev-api-rebuild:
	@echo "Rebuilding and starting api service (docker)..."
	@docker compose -f $(DEV_COMPOSE_FILE) up -d --build $(DEV_API_SERVICE)

dev-web:
	@echo "Starting web frontend dev server..."
	@echo "Web frontend: http://localhost:$(DEV_WEB_PORT)"
	@cd $(WEB_DIR) && bun install
	@cd $(WEB_DIR) && bun run dev -- --host 0.0.0.0 --port $(DEV_WEB_PORT)

dev: dev-api dev-web

# The main package embeds the ignored web/dist output and is covered after build-web.
test:
	@echo "Testing root Go module..."
	@root_module=$$(GOWORK=off go list -m); \
		root_packages=$$(GOWORK=off go list -e ./... | grep -vxF "$$root_module"); \
		GOWORK=off go test -race -count=1 $$root_packages
	@echo "Testing relaykit Go module..."
	@cd relaykit && GOWORK=off go test -race -count=1 ./...

swag:
	swag init -g controller/swagger.go --parseDependency --parseInternal -o docs

reset-setup:
	@echo "Resetting local setup wizard state..."
	@if docker compose -f $(DEV_COMPOSE_FILE) ps --services --status running | grep -qx "$(DEV_POSTGRES_SERVICE)"; then \
		echo "Detected running docker dev PostgreSQL. Removing setup record and root users..."; \
		docker compose -f $(DEV_COMPOSE_FILE) exec -T $(DEV_POSTGRES_SERVICE) \
			psql -U $(DEV_POSTGRES_USER) -d $(DEV_POSTGRES_DB) \
			-c 'DELETE FROM setups;' \
			-c 'DELETE FROM users WHERE role = 100;' \
			-c "DELETE FROM options WHERE key IN ('SelfUseModeEnabled', 'DemoSiteEnabled');"; \
		echo "Restarting docker dev api so setup status is recalculated..."; \
		docker compose -f $(DEV_COMPOSE_FILE) restart $(DEV_API_SERVICE); \
	elif db_path="$${SQLITE_PATH:-$(DEV_SQLITE_PATH)}"; db_path="$${db_path%%\?*}"; [ -f "$$db_path" ]; then \
		db_path="$${SQLITE_PATH:-$(DEV_SQLITE_PATH)}"; \
		db_path="$${db_path%%\?*}"; \
		echo "Detected local SQLite database: $$db_path"; \
		sqlite3 "$$db_path" \
			"DELETE FROM setups; DELETE FROM users WHERE role = 100; DELETE FROM options WHERE key IN ('SelfUseModeEnabled', 'DemoSiteEnabled');"; \
		echo "SQLite setup state reset. Restart the local api process before testing the setup wizard."; \
	else \
		echo "No running docker dev PostgreSQL or local SQLite database found."; \
		echo "Start the dev stack with 'make dev-api', or set SQLITE_PATH/DEV_SQLITE_PATH to your local SQLite database."; \
		exit 1; \
	fi

# Offline deployment package: pull every image referenced by the compose stack
# (docker-compose.yml + docker-compose.docs.yml) and bundle them together with
# the deployment files into a single gzipped tarball $(OFFLINE_PACKAGE), so a
# machine without any registry access can deploy by extracting the archive and
# running `docker load -i images.tar` + `docker compose up -d`.
# Archive layout (flat, so `tar xzf` yields a ready-to-use deploy directory):
#   images.tar            docker save output of all stack images
#   docker-compose.yml    core services (cuberouter / postgres / redis)
#   docker-compose.docs.yml  documentation sites
#   deploy.md             deployment guide
#   scripts/gen-tls-cert.sh  HTTPS certificate generator
# CUBEROUTER_IMAGE_TAG pins the versioned images
# (e.g. CUBEROUTER_IMAGE_TAG=v1.0.0 make offline-package).
# The archive is staged as temp files and atomically renamed into place, so a
# failed docker save/tar never leaves a partial $(OFFLINE_PACKAGE) behind.
offline-package:
	@set -e; \
	images=$$(docker compose -f docker-compose.yml -f docker-compose.docs.yml config --images | sort -u); \
	echo "Packaging images into $(OFFLINE_PACKAGE):"; \
	echo "$$images"; \
	for img in $$images; do \
		echo "pulling $$img"; \
		docker pull "$$img"; \
	done; \
	tmp_dir="$$(mktemp -d)"; \
	tmp_gz="$(OFFLINE_PACKAGE).tmp.gz"; \
	trap 'rm -rf "$$tmp_dir"; rm -f "$$tmp_gz"' EXIT; \
	echo "saving $$images -> images.tar"; \
	if ! docker save $$images > "$$tmp_dir/images.tar"; then \
		echo "docker save failed; discarding partial archive" >&2; \
		exit 1; \
	fi; \
	mkdir -p "$$tmp_dir/scripts"; \
	cp docker-compose.yml docker-compose.docs.yml deploy.md "$$tmp_dir/"; \
	cp scripts/gen-tls-cert.sh "$$tmp_dir/scripts/"; \
	sed -i "s|:\$${CUBEROUTER_IMAGE_TAG:-latest}|:$(CUBEROUTER_IMAGE_TAG)|g" \
		"$$tmp_dir/docker-compose.yml" "$$tmp_dir/docker-compose.docs.yml"; \
	if ! tar -C "$$tmp_dir" -czf "$$tmp_gz" .; then \
		echo "packaging failed; discarding partial archive" >&2; \
		exit 1; \
	fi; \
	mv "$$tmp_gz" "$(OFFLINE_PACKAGE)"; \
	echo "Offline package ready: $(OFFLINE_PACKAGE)"; \
	echo "On the target machine: tar xzf $(OFFLINE_PACKAGE) && docker load -i images.tar && docker compose up -d && docker compose -f docker-compose.docs.yml up -d"
