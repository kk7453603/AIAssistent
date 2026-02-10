.PHONY: generate-openapi generate test

generate-openapi:
	go generate ./internal/adapters/http/openapi

generate: generate-openapi

test:
	go test ./...
