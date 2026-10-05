package scaffold

const dockerfileTemplate = `
FROM golang:{{ .GoVersion }}-alpine AS builder

WORKDIR /src
RUN apk add --no-cache ca-certificates tzdata
COPY . .
RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/api

FROM alpine:3.20 AS runtime
RUN addgroup -S app && adduser -S -G app app && apk add --no-cache ca-certificates wget
WORKDIR /app
COPY --from=builder /out/app /app/app
USER app
EXPOSE {{ .AppPort }}
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD wget --quiet --tries=1 --spider http://127.0.0.1:{{ .AppPort }}/healthz || exit 1
ENTRYPOINT ["/app/app"]
`

const dockerComposeTemplate = `
services:
  app:
    build:
      context: ..
      dockerfile: deploy/Dockerfile
    env_file:
      - ../.env
    restart: unless-stopped
{{- if .HasObservability }}
    labels:
      logging: "promtail"
{{- end }}
    ports:
      - "${APP_PORT:-{{ .AppPort }}}:{{ .AppPort }}"
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://127.0.0.1:{{ .AppPort }}/healthz"]
      interval: 30s
      timeout: 3s
      retries: 3
      start_period: 10s
{{- if or .HasPostgres .HasRedis }}
    depends_on:
{{- if .HasPostgres }}
      postgres:
        condition: service_healthy
{{- end }}
{{- if .HasRedis }}
      redis:
        condition: service_healthy
{{- end }}
{{- end }}

{{- if .HasPostgres }}
  postgres:
    image: postgres:16
    restart: unless-stopped
    environment:
      POSTGRES_DB: {{ .ProjectName }}
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres -d {{ .ProjectName }}"]
      interval: 10s
      timeout: 5s
      retries: 5
{{- end }}

{{- if .HasRedis }}
  redis:
    image: redis:7
    restart: unless-stopped
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 3s
      retries: 5
{{- end }}

{{- if .HasObservability }}
  otel-collector:
    image: otel/opentelemetry-collector:0.115.1
    restart: unless-stopped
    command: ["--config=/etc/otelcol/config.yaml"]
    volumes:
      - ./otelcol/config.yaml:/etc/otelcol/config.yaml:ro
    ports:
      - "4317:4317"
      - "4318:4318"
      - "8889:8889"

  prometheus:
    image: prom/prometheus:v2.54.1
    restart: unless-stopped
    volumes:
      - ./prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
    ports:
      - "9090:9090"

  loki:
    image: grafana/loki:3.3.2
    restart: unless-stopped
    command: -config.file=/etc/loki/local-config.yaml
    volumes:
      - ./loki/config.yaml:/etc/loki/local-config.yaml:ro
      - loki_data:/loki
    ports:
      - "3100:3100"

  promtail:
    image: grafana/promtail:3.3.2
    restart: unless-stopped
    command: -config.file=/etc/promtail/config.yaml
    volumes:
      - ./promtail/config.yaml:/etc/promtail/config.yaml:ro
      - /var/lib/docker/containers:/var/lib/docker/containers:ro
      - /var/run/docker.sock:/var/run/docker.sock:ro
    depends_on:
      loki:
        condition: service_started

  tempo:
    image: grafana/tempo:2.6.0
    restart: unless-stopped
    command: ["-config.file=/etc/tempo.yaml"]
    volumes:
      - ./tempo/tempo-config.yaml:/etc/tempo.yaml:ro
      - tempo_data:/var/tempo
    ports:
      - "3200:3200"

  grafana:
    image: grafana/grafana:11.1.4
    restart: unless-stopped
    environment:
      GF_SECURITY_ADMIN_USER: ${GRAFANA_ADMIN_USER:-admin}
      GF_SECURITY_ADMIN_PASSWORD: ${GRAFANA_ADMIN_PASSWORD:-admin}
      GF_USERS_ALLOW_SIGN_UP: "false"
    volumes:
      - grafana_data:/var/lib/grafana
      - ./grafana/provisioning/dashboards:/etc/grafana/provisioning/dashboards:ro
      - ./grafana/provisioning/datasources:/etc/grafana/provisioning/datasources:ro
    ports:
      - "3000:3000"
{{- end }}

{{- if or .HasPostgres .HasObservability }}
volumes:
{{- if .HasPostgres }}
  postgres_data:
{{- end }}
{{- if .HasObservability }}
  grafana_data:
  loki_data:
  tempo_data:
{{- end }}
{{- end }}
`

const otelCollectorTemplate = `
receivers:
  otlp:
    protocols:
      grpc:
      http:

exporters:
  debug: {}
  otlp/tempo:
    endpoint: tempo:4317
    tls:
      insecure: true
  prometheus:
    endpoint: "0.0.0.0:8889"

processors:
  batch: {}

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlp/tempo, debug]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [prometheus, debug]
`

const prometheusTemplate = `
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: app
    metrics_path: /metrics
    static_configs:
      - targets: ["app:{{ .AppPort }}"]

  - job_name: otel-collector
    static_configs:
      - targets: ["otel-collector:8889"]

  - job_name: prometheus
    static_configs:
      - targets: ["prometheus:9090"]
`

const lokiTemplate = `
auth_enabled: false

server:
  http_listen_port: 3100
  grpc_listen_port: 9096

common:
  storage:
    filesystem:
      directory: /loki/chunks
  replication_factor: 1
  ring:
    instance_addr: 127.0.0.1
    kvstore:
      store: inmemory

ingester:
  lifecycler:
    ring:
      kvstore:
        store: inmemory
      replication_factor: 1
    final_sleep: 0s
  chunk_idle_period: 1h
  max_chunk_age: 1h
  chunk_target_size: 1048576
  chunk_retain_period: 30s
  wal:
    enabled: true
    dir: /loki/wal

schema_config:
  configs:
    - from: 2020-10-24
      store: boltdb-shipper
      object_store: filesystem
      schema: v11
      index:
        prefix: index_
        period: 24h

storage_config:
  boltdb_shipper:
    active_index_directory: /loki/index
    cache_location: /loki/index_cache
    cache_ttl: 24h
  filesystem:
    directory: /loki/chunks

compactor:
  working_directory: /loki/compactor
  shared_store: filesystem

ruler:
  storage:
    type: local
    local:
      directory: /loki/rules
  rule_path: /loki/rules-temp
  ring:
    kvstore:
      store: inmemory
  enable_api: true
`

const promtailTemplate = `
clients:
  - url: http://loki:3100/loki/api/v1/push

scrape_configs:
  - job_name: docker
    docker_sd_configs:
      - host: unix:///var/run/docker.sock
        refresh_interval: 5s
        filters:
          - name: label
            values: ["logging=promtail"]
    relabel_configs:
      - source_labels: ["__meta_docker_container_name"]
        regex: "/(.*)"
        target_label: "container"
    pipeline_stages:
      - json:
          expressions:
            level: level
      - labels:
          level:
`

const tempoTemplate = `
server:
  http_listen_port: 3200
  grpc_listen_port: 4317

distributor:
  receivers:
    otlp:
      protocols:
        grpc:
          endpoint: 0.0.0.0:4317
        http:
          endpoint: 0.0.0.0:4318

ingester:
  trace_idle_period: 10s
  max_block_duration: 5m

compactor:
  compaction:
    block_retention: 1h

storage:
  trace:
    backend: local
    local:
      path: /var/tempo/traces
`

const grafanaDatasourceTemplate = `
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    orgId: 1
    url: http://prometheus:9090
    basicAuth: false
    isDefault: true
    editable: true

  - name: Loki
    type: loki
    access: proxy
    orgId: 1
    url: http://loki:3100
    basicAuth: false
    editable: true

  - name: Tempo
    type: tempo
    access: proxy
    orgId: 1
    url: http://tempo:3200
    basicAuth: false
    editable: true
`

const grafanaDashboardProviderTemplate = `
apiVersion: 1

providers:
  - name: App Overview
    orgId: 1
    folder: ''
    type: file
    disableDeletion: false
    editable: true
    allowUiUpdates: true
    options:
      path: /etc/grafana/provisioning/dashboards
`

const grafanaAppDashboardTemplate = `
{
  "annotations": {
    "list": [
      {
        "builtIn": 1,
        "datasource": {
          "type": "grafana",
          "uid": "-- Grafana --"
        },
        "enable": true,
        "hide": true,
        "iconColor": "rgba(0, 211, 255, 1)",
        "name": "Annotations & Alerts",
        "type": "dashboard"
      }
    ]
  },
  "editable": true,
  "panels": [
    {
      "datasource": {
        "type": "prometheus",
        "uid": "${DS_PROMETHEUS}"
      },
      "fieldConfig": {
        "defaults": {
          "color": {
            "mode": "palette-classic"
          },
          "unit": "reqps"
        },
        "overrides": []
      },
      "gridPos": {
        "h": 8,
        "w": 24,
        "x": 0,
        "y": 0
      },
      "id": 1,
      "targets": [
        {
          "expr": "rate(http_requests_total[5m])",
          "legendFormat": "request-rate",
          "refId": "A"
        }
      ],
      "title": "HTTP Request Rate",
      "type": "timeseries"
    }
  ],
  "schemaVersion": 39,
  "style": "dark",
  "tags": ["generated", "chi", "observability"],
  "templating": {
    "list": []
  },
  "time": {
    "from": "now-30m",
    "to": "now"
  },
  "title": "App Overview",
  "uid": "app-overview",
  "version": 1
}
`
