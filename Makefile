.PHONY: generate build run tidy

generate:
	sqlc generate

build:
	go build -o bin/vozzera ./cmd/api

run:
	go run ./cmd/api

tidy:
	go mod tidy
