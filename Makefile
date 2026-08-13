# Configurações de Banco de Dados
DB_URL ?= "postgres://admin:admin@localhost:5432/vozzera?sslmode=disable"
MIGRATIONS_DIR = sql/migrations

.PHONY: generate build run tidy migrate-create migrate-up migrate-down migrate-status

generate:
	go tool sqlc generate

build:
	go build -o bin/vozzera ./cmd/api

run:
	go run -race ./cmd/api

tidy:
	go mod tidy

# Criar arquivo de migration. Uso: make migrate-create name=<nome_da_migration>
migrate-create:
	go tool goose -dir $(MIGRATIONS_DIR) create $(name) sql

# Aplicar migrations no banco
migrate-up:
	go tool goose -dir $(MIGRATIONS_DIR) postgres $(DB_URL) up

# Reverter a última migration
migrate-down:
	go tool goose -dir $(MIGRATIONS_DIR) postgres $(DB_URL) down

# Verificar o estado atual do banco
migrate-status:
	go tool goose -dir $(MIGRATIONS_DIR) postgres $(DB_URL) status