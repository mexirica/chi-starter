.PHONY: help run build test tidy

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  run    Run the Chi Template app"
	@echo "  build  Build the Chi Template binary"
	@echo "  test   Run the test suite"
	@echo "  tidy   Sync Go module files"

run:
	@go run ./cmd

build:
	@mkdir -p bin
	@go build -o bin/chi-template ./cmd

test:
	@go test -v ./...

tidy:
	@go mod tidy