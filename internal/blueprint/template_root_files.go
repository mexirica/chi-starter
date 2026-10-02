package blueprint

const gitignoreTemplate = `
# Binaries and build output
bin/
dist/
tmp/

# Local configuration
.env
.air.toml

# Test and coverage output
*.test
*.out
coverage.out

# Go workspace files
go.work

# IDE and OS files
.vscode/
.idea/
.DS_Store
`

const envTemplate = `
APP_PORT={{ .AppPort }}
APP_ENV=development
LOG_LEVEL=info
HTTP_REQUEST_TIMEOUT=30s
HTTP_READ_HEADER_TIMEOUT=5s
HTTP_READ_TIMEOUT=10s
HTTP_WRITE_TIMEOUT=15s
HTTP_IDLE_TIMEOUT=60s
APP_SHUTDOWN_TIMEOUT=10s
{{- if .HasPostgres }}
DATABASE_URL=postgres://postgres:postgres@localhost:5432/{{ .ProjectName }}?sslmode=disable
{{- end }}
{{- if .HasRedis }}
REDIS_ADDR=localhost:6379
{{- end }}
{{- if .HasObservability }}
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=admin
{{- end }}
`

const airTemplate = `
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
	args_bin = []
	bin = "./tmp/app"
	cmd = "go build -o ./tmp/app ./cmd/api"
	delay = 1000
	exclude_dir = ["assets", "deploy", "tmp", "vendor", "testdata"]
	exclude_regex = ["_test.go"]
	include_ext = ["go", "tpl", "tmpl", "html"]
	log = "build-errors.log"
	stop_on_error = true

[color]
	app = "green"
	build = "yellow"
	main = "magenta"
	runner = "cyan"
	watcher = "blue"
`

const readmeTemplate = `
# {{ .ProjectName }}

Generated with Chi Template.

Production-first baseline for Go + Chi services.

## Enabled Blocks

- core (always included)
{{- if .SelectedBlockList }}
{{- range .SelectedBlockList }}
- {{ . }}
{{- end }}
{{- else }}
- No optional block selected
{{- end }}

## Project Layout

- cmd/api: application entrypoint
- internal/app: lifecycle and dependency wiring
- internal/server: router, middleware, probes, tests
- internal/config: env configuration and validation
- internal/logging: structured logging
- internal/helpers: reusable HTTP/JSON helpers
- internal/validation: payload binding and validation
{{- if .HasPostgres }}
- internal/database: Postgres integration
{{- end }}
{{- if .HasRedis }}
- internal/cache: Redis integration helpers
- internal/middleware: optional Redis-backed HTTP cache middleware
{{- end }}
{{- if .HasObservability }}
- internal/o11y: OTLP bootstrap + metrics exposure
- deploy/otelcol, deploy/prometheus, deploy/loki, deploy/promtail, deploy/tempo
- deploy/grafana/provisioning: datasources and dashboards
{{- end }}
{{- if .HasDocker }}
- deploy: local container stack files
{{- end }}

## Quick Start

1. Run make tidy.
2. Run make ci.
3. Run make run.
{{- if .HasDocker }}
4. To run local dependencies with Docker, use make compose-up.
{{- end }}

## Common Commands

- make build: build the application
- make run: run the API locally
- make test: run test suite
- make test-cover: run test suite with coverage
- make test-race: run race detector
- make fmt: format Go code
- make vet: run static checks
- make tidy: sync go.mod and go.sum
- make ci: run local quality pipeline
- make watch: run live reload with air
{{- if .HasDocker }}
- make compose-up: start local stack with Docker Compose
- make compose-down: stop local stack
- make compose-logs: stream stack logs
{{- end }}

## Health Endpoints

- GET /healthz: liveness probe
- GET /readyz: readiness probe with optional dependency checks
- GET /v1/healthz and /v1/readyz: versioned probe endpoints
{{- if .HasObservability }}
- GET /metrics: Prometheus metrics
{{- end }}

## Environment

- APP_ENV sets the running environment (development, staging, production)
- LOG_LEVEL controls logger level (debug, info, warn, error)
- HTTP_REQUEST_TIMEOUT and related HTTP timeout vars tune request lifecycle
- APP_SHUTDOWN_TIMEOUT controls graceful shutdown timeout

{{- if .HasObservability }}
## Local Observability

- Grafana: http://localhost:3000
- Prometheus: http://localhost:9090
- Loki: http://localhost:3100
- Tempo: http://localhost:3200
{{- end }}
`

const makefileTemplate = `
.PHONY: all ci build run test test-cover test-race fmt vet tidy watch compose-up compose-down compose-logs

all: fmt vet test build

ci: fmt vet test-race build

build:
	@mkdir -p bin
	@go build -o bin/{{ .ProjectName }} ./cmd/api

run:
	@go run ./cmd/api

test:
	@go test ./...

test-cover:
	@go test -cover ./...

test-race:
	@go test -race ./...

fmt:
	gofmt -w ./cmd ./internal

vet:
	@go vet ./...

tidy:
	@go mod tidy

watch:
	air
{{- if .HasDocker }}

compose-up:
	docker compose -f deploy/docker-compose.yml --env-file .env up --build -d

compose-down:
	docker compose -f deploy/docker-compose.yml down -v

compose-logs:
	docker compose -f deploy/docker-compose.yml logs -f
{{- end }}
`

const goModTemplate = `
module {{ .ModulePath }}

go {{ .GoVersion }}

require (
{{- range .GoRequirements }}
	{{ . }}
{{- end }}
)
`
