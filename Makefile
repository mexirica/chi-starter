.PHONY: help run build test tidy

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  run    Run the Bubble Tea blueprint builder"
	@echo "  build  Build the blueprint builder binary"
	@echo "  test   Run the test suite"
	@echo "  tidy   Sync Go module files"

run:
	@go run ./cmd

build:
	@mkdir -p bin
	@go build -o bin/chi-blueprint ./cmd

test:
	@go test -v ./...

tidy:
	@go mod tidy