.PHONY: help test lint build clean coverage

help:
	@echo "Available targets:"
	@echo "  test      - Run tests"
	@echo "  lint      - Run linter"
	@echo "  build     - Build the project"
	@echo "  clean     - Clean build artifacts"
	@echo "  coverage  - Run tests with coverage report"

test:
	go test -v -race ./...

lint:
	golangci-lint run

build:
	go build -v ./...

clean:
	go clean -cache -testcache -modcache
	rm -f coverage.out

coverage:
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
