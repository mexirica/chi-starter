.PHONY: help run build test tidy

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  run    Run the Chi Starter app"
	@echo "  build  Build the Chi Starter binary"
	@echo "  test   Run the test suite"
	@echo "  tidy   Sync Go module files"

run:
	@go run ./cmd

build:
	@mkdir -p bin
	@go build -o bin/chi-starter ./cmd

test:
	@go test -v ./...

tidy:
	@go mod tidy