# CodeArena — developer entrypoints. Run `make` (or `make help`) for a list.

DATABASE_URL   ?= postgres://codearena:codearena@localhost:5432/codearena?sslmode=disable
KAFKA_BROKERS  ?= localhost:9094
JWT_SECRET     ?= dev-secret-change-me
MIGRATIONS_DIR ?= ./migrations
RUN_TIMEOUT_MS ?= 10000

.DEFAULT_GOAL := help

.PHONY: help dev-infra run-server run-worker up down build frontend test runner-image k8s-apply k8s-delete psql

help: ## Show this help
	@echo "CodeArena targets:"
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "} {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

dev-infra: ## Start just postgres + kafka in docker (for local go run dev)
	docker compose up -d postgres kafka

run-server: ## Run the API server locally against dev-infra
	PORT=8080 \
	DATABASE_URL="$(DATABASE_URL)" \
	KAFKA_BROKERS="$(KAFKA_BROKERS)" \
	JWT_SECRET="$(JWT_SECRET)" \
	MIGRATIONS_DIR="$(MIGRATIONS_DIR)" \
	go run ./cmd/server

run-worker: ## Run the run worker locally (local executor, no k8s needed)
	DATABASE_URL="$(DATABASE_URL)" \
	KAFKA_BROKERS="$(KAFKA_BROKERS)" \
	EXECUTOR=local \
	RUN_TIMEOUT_MS="$(RUN_TIMEOUT_MS)" \
	go run ./cmd/worker

up: ## Build and start the full stack (postgres, kafka, server, worker)
	docker compose up --build

down: ## Stop the stack (keeps volumes; add -v manually to wipe data)
	docker compose down

build: ## Compile server and worker binaries into ./bin
	mkdir -p bin
	go build -o bin/server ./cmd/server
	go build -o bin/worker ./cmd/worker

frontend: ## Build the React frontend into ./web (served by the Go server)
	cd frontend && npm install && npm run build

test: ## Run the Go test suite
	go test ./...

runner-image: ## Build the sandbox execution image used by k8s Job pods
	docker build -t codearena-runner:latest ./runner

k8s-apply: ## Apply all Kubernetes manifests (namespace, db, kafka, apps, rbac)
	kubectl apply -f deploy/k8s/

k8s-delete: ## Delete all CodeArena Kubernetes resources
	kubectl delete -f deploy/k8s/

psql: ## Open a psql shell into the compose postgres
	docker compose exec postgres psql -U codearena
