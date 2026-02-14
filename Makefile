.PHONY: generate-openapi generate test vet test-core-cover

generate-openapi:
	go generate ./internal/adapters/http/openapi

generate: generate-openapi

test:
	go test ./...

vet:
	go vet ./...

test-core-cover:
	go test ./internal/core/... ./internal/adapters/http -coverprofile=coverage.out
