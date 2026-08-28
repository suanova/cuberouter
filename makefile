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
# (docker-compose.yml + docker-compose.docs.yml) and save them all as a single
# gzipped tarball $(OFFLINE_PACKAGE). CUBEROUTER_IMAGE_TAG pins the versioned
# images (e.g. CUBEROUTER_IMAGE_TAG=v1.0.0 make offline-package).
# The archive is staged as temp files and atomically renamed into place, so a
# failed docker save/gzip never leaves a partial $(OFFLINE_PACKAGE) behind.
offline-package:
	@images=$$(docker compose -f docker-compose.yml -f docker-compose.docs.yml config --images | sort -u); \
	echo "Packaging images into $(OFFLINE_PACKAGE):"; \
	echo "$$images"; \
	for img in $$images; do \
		echo "pulling $$img"; \
		docker pull "$$img"; \
	done; \
	echo "saving $$images -> $(OFFLINE_PACKAGE)"; \
	tmp_tar="$(OFFLINE_PACKAGE).tmp.tar"; \
	tmp_gz="$(OFFLINE_PACKAGE).tmp.gz"; \
	trap 'rm -f "$$tmp_tar" "$$tmp_gz"' EXIT; \
	if ! docker save $$images > "$$tmp_tar"; then \
		echo "docker save failed; discarding partial archive" >&2; \
		exit 1; \
	fi; \
	if ! gzip -c "$$tmp_tar" > "$$tmp_gz"; then \
		echo "gzip failed; discarding partial archive" >&2; \
		exit 1; \
	fi; \
	rm -f "$$tmp_tar"; \
	mv "$$tmp_gz" "$(OFFLINE_PACKAGE)"; \
	echo "Offline package ready: $(OFFLINE_PACKAGE)"
