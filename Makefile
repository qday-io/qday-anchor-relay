SHELL := /bin/bash

.PHONY: up down logs migrate test test-relay test-contracts abigen portal-dev

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f

# Run migrations against a locally reachable Postgres (compose runs them automatically).
migrate:
	docker compose run --rm migrate

test: test-relay test-contracts

test-relay:
	cd relay && go test ./...

test-contracts:
	cd contracts && forge test -vv

# Regenerate the Go binding for QDayAnchorRegistry from the Foundry build artifact.
abigen:
	cd contracts && forge build
	jq -r '.abi' contracts/out/QDayAnchorRegistry.sol/QDayAnchorRegistry.json > contracts/out/QDayAnchorRegistry.abi.json
	go run github.com/ethereum/go-ethereum/cmd/abigen@v1.14.13 \
		--abi contracts/out/QDayAnchorRegistry.abi.json \
		--pkg registry --type QDayAnchorRegistry \
		--out relay/internal/registry/qday_anchor_registry.go

portal-dev:
	cd portal && npm run dev
