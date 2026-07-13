SHELL := /bin/bash

# Local Go builds need the cryptosuite liboqs FORK via pkg-config (CGO).
# Set LIBOQS_PREFIX to your install prefix; see README "Building liboqs".
LIBOQS_PREFIX ?=
ifneq ($(LIBOQS_PREFIX),)
export PKG_CONFIG_PATH := $(LIBOQS_PREFIX)/lib/pkgconfig:$(PKG_CONFIG_PATH)
export CGO_LDFLAGS := -Wl,-rpath,$(LIBOQS_PREFIX)/lib
endif

.PHONY: up down logs migrate test test-relay test-contracts abigen portal-dev

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f

# Run migrations against a locally reachable Postgres (compose runs them automatically).
# Migrations are embedded in the relay binary and applied automatically at
# startup. This target only exists for applying them to an arbitrary DB
# (e.g. the local test postgres) without starting the relay.
migrate:
	docker run --rm -v $(PWD)/relay/migrations:/migrations:ro migrate/migrate:v4.18.1 \
		-path=/migrations -database "$(DATABASE_URL)" up

test: test-relay test-contracts

test-relay:
	cd relay && go test ./...

test-contracts:
	cd contracts && forge test -vv

# Regenerate the Go binding for QDayAnchorRegistry from the Foundry build artifact.
abigen:
	cd contracts && forge build
	cd contracts && forge inspect src/QDayAnchorRegistry.sol:QDayAnchorRegistry abi --json > out/QDayAnchorRegistry.abi.json
	cd contracts && forge inspect src/QDayAnchorRegistry.sol:QDayAnchorRegistry bytecode > out/QDayAnchorRegistry.bin
	cd relay && go run github.com/ethereum/go-ethereum/cmd/abigen@v1.14.13 \
		--abi ../contracts/out/QDayAnchorRegistry.abi.json \
		--bin ../contracts/out/QDayAnchorRegistry.bin \
		--pkg registry --type QDayAnchorRegistry \
		--out internal/registry/qday_anchor_registry.go

portal-dev:
	cd portal && npm run dev
