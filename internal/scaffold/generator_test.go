package scaffold

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCoreProject(t *testing.T) {
	t.Parallel()

	outputDir := filepath.Join(t.TempDir(), "core-project")
	result, err := Generate(Options{
		ProjectName: "core-project",
		ModulePath:  "github.com/example/core-project",
		OutputDir:   outputDir,
		Blocks:      map[string]bool{},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(result.Files) == 0 {
		t.Fatal("expected generated files")
	}

	routerBytes, err := os.ReadFile(filepath.Join(outputDir, "internal/server/routes.go"))
	if err != nil {
		t.Fatalf("read router: %v", err)
	}

	router := string(routerBytes)
	if !strings.Contains(router, "type RouterOptions struct") {
		t.Fatal("expected router to expose RouterOptions for extensible server wiring")
	}
	if !strings.Contains(router, "router.Route(\"/v1\"") {
		t.Fatal("expected generated router to expose versioned routes")
	}
	if !strings.Contains(router, "requestLogger(logger, metrics)") {
		t.Fatal("expected generated router to include structured request logging middleware")
	}

	configBytes, err := os.ReadFile(filepath.Join(outputDir, "internal/config/config.go"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	config := string(configBytes)
	if !strings.Contains(config, "HTTPRequestTimeout") {
		t.Fatal("expected generated config to include typed HTTP timeout fields")
	}
	if !strings.Contains(config, "durationOrDefault(\"HTTP_REQUEST_TIMEOUT\", \"30s\")") {
		t.Fatal("expected generated config to parse duration-based settings")
	}
	if !strings.Contains(config, "func isSupportedLogLevel(level string) bool") {
		t.Fatal("expected generated config to validate log levels")
	}

	goModBytes, err := os.ReadFile(filepath.Join(outputDir, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}

	goMod := string(goModBytes)
	expectedGoVersion := detectGoVersion()
	if !strings.Contains(goMod, "\ngo "+expectedGoVersion+"\n") {
		t.Fatalf("expected generated go.mod to use local Go version %q", expectedGoVersion)
	}

	makefileBytes, err := os.ReadFile(filepath.Join(outputDir, "Makefile"))
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}

	makefile := string(makefileBytes)
	for _, expected := range []string{
		"\t@go build -o bin/",
		"\t@go run ./cmd/api",
		"\t@go test ./...",
		"\t@go test -cover ./...",
		"\t@go test -race ./...",
		"\t@go vet ./...",
		"\t@go mod tidy",
	} {
		if !strings.Contains(makefile, expected) {
			t.Fatalf("expected generated Makefile to contain %q", expected)
		}
	}

	if strings.Contains(makefile, "\n  @go") {
		t.Fatal("expected generated Makefile to avoid space-indented @go recipes")
	}

	mainBytes, err := os.ReadFile(filepath.Join(outputDir, "cmd/api/main.go"))
	if err != nil {
		t.Fatalf("read main: %v", err)
	}

	mainFile := string(mainBytes)
	if !strings.Contains(mainFile, "application, err := app.New(ctx, cfg)") {
		t.Fatal("expected generated main to bootstrap the app package")
	}
	if !strings.Contains(mainFile, "application.Shutdown(nil)") {
		t.Fatal("expected generated main to rely on app-level shutdown timeout defaults")
	}

	appBytes, err := os.ReadFile(filepath.Join(outputDir, "internal/app/app.go"))
	if err != nil {
		t.Fatalf("read app: %v", err)
	}

	appFile := string(appBytes)
	for _, expected := range []string{"type App struct", "shutdownTimeout time.Duration", "slog.SetDefault(logger)", "server.RouterOptions", "func New(ctx context.Context, cfg config.Config)", "func (application *App) Run() error", "func (application *App) Shutdown(ctx context.Context) error"} {
		if !strings.Contains(appFile, expected) {
			t.Fatalf("expected generated app file to contain %q", expected)
		}
	}

	metricsBytes, err := os.ReadFile(filepath.Join(outputDir, "internal/server/metrics.go"))
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}

	metrics := string(metricsBytes)
	if !strings.Contains(metrics, "type noopHTTPMetrics struct{}") {
		t.Fatal("expected generated metrics file to include noop implementation for core projects")
	}

	for _, path := range []string{
		".air.toml",
		"cmd/api/main.go",
		"internal/app/app.go",
		"internal/helpers/helpers.go",
		"internal/logging/logging.go",
		"internal/server/routes.go",
		"internal/server/health.go",
		"internal/server/middleware.go",
		"internal/server/metrics.go",
		"internal/server/server.go",
		"internal/server/routes_test.go",
		"internal/validation/validation.go",
	} {
		if _, err := os.Stat(filepath.Join(outputDir, path)); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}

	for _, path := range []string{
		"internal/api/router.go",
		"internal/platform/database/postgres.go",
		"internal/platform/cache/redis.go",
	} {
		if _, err := os.Stat(filepath.Join(outputDir, path)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be absent, got err=%v", path, err)
		}
	}

	if _, err := os.Stat(filepath.Join(outputDir, "deploy", "docker-compose.yml")); !os.IsNotExist(err) {
		t.Fatalf("expected no docker compose file for core-only project, got err=%v", err)
	}

	buildGeneratedProject(t, outputDir)
}

func TestGenerateAllBlocks(t *testing.T) {
	t.Parallel()

	outputDir := filepath.Join(t.TempDir(), "full-project")
	result, err := Generate(Options{
		ProjectName: "full-project",
		ModulePath:  "github.com/example/full-project",
		OutputDir:   outputDir,
		Blocks: map[string]bool{
			"postgres":      true,
			"redis":         true,
			"observability": true,
			"docker":        true,
		},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(result.Files) < 10 {
		t.Fatalf("expected a richer output for all blocks, got %d files", len(result.Files))
	}

	composeBytes, err := os.ReadFile(filepath.Join(outputDir, "deploy", "docker-compose.yml"))
	if err != nil {
		t.Fatalf("read docker compose: %v", err)
	}

	compose := string(composeBytes)
	for _, expected := range []string{"postgres:", "redis:", "otel-collector:", "prometheus:", "grafana:", "loki:", "promtail:", "tempo:", "restart: unless-stopped", "healthcheck:", "env_file:", "logging: \"promtail\""} {
		if !strings.Contains(compose, expected) {
			t.Fatalf("expected docker compose to contain %q", expected)
		}
	}

	if strings.Contains(compose, "\t") {
		t.Fatal("expected docker compose to use spaces instead of tabs")
	}
	appService := strings.SplitN(compose, "\n  postgres:\n", 2)[0]
	for _, dependency := range []string{"otel-collector:", "prometheus:", "loki:", "tempo:", "grafana:"} {
		if strings.Contains(appService, dependency) {
			t.Fatalf("expected app service not to depend on observability service %q", dependency)
		}
	}

	for _, path := range []string{
		".air.toml",
		"internal/app/app.go",
		"internal/helpers/helpers.go",
		"internal/logging/logging.go",
		"internal/middleware/cache.go",
		"internal/middleware/cache_test.go",
		"internal/database/postgres.go",
		"internal/cache/redis.go",
		"internal/o11y/o11y.go",
		"internal/server/health.go",
		"internal/server/middleware.go",
		"internal/server/metrics.go",
		"internal/validation/validation.go",
		"deploy/otelcol/config.yaml",
		"deploy/prometheus/prometheus.yml",
		"deploy/loki/config.yaml",
		"deploy/promtail/config.yaml",
		"deploy/tempo/tempo-config.yaml",
		"deploy/grafana/provisioning/datasources/datasource.yml",
		"deploy/grafana/provisioning/dashboards/dashboard.yml",
		"deploy/grafana/provisioning/dashboards/app_overview.json",
	} {
		if _, err := os.Stat(filepath.Join(outputDir, path)); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}

	otelConfigBytes, err := os.ReadFile(filepath.Join(outputDir, "deploy", "otelcol", "config.yaml"))
	if err != nil {
		t.Fatalf("read otel collector config: %v", err)
	}

	otelConfig := string(otelConfigBytes)
	for _, expected := range []string{"otlp/tempo", "prometheus:", "metrics:", "endpoint: \"0.0.0.0:8889\""} {
		if !strings.Contains(otelConfig, expected) {
			t.Fatalf("expected otel collector config to contain %q", expected)
		}
	}

	grafanaDatasourceBytes, err := os.ReadFile(filepath.Join(outputDir, "deploy", "grafana", "provisioning", "datasources", "datasource.yml"))
	if err != nil {
		t.Fatalf("read grafana datasource config: %v", err)
	}

	grafanaDatasource := string(grafanaDatasourceBytes)
	for _, expected := range []string{"name: Prometheus", "name: Loki", "name: Tempo"} {
		if !strings.Contains(grafanaDatasource, expected) {
			t.Fatalf("expected grafana datasource config to contain %q", expected)
		}
	}

	cacheMiddlewareBytes, err := os.ReadFile(filepath.Join(outputDir, "internal", "middleware", "cache.go"))
	if err != nil {
		t.Fatalf("read cache middleware: %v", err)
	}

	cacheMiddleware := string(cacheMiddlewareBytes)
	for _, expected := range []string{"func Cache(", "isCacheableRequest(request)", "isCacheableResponse(recorder.Header())", "Authorization", "Cache-Control", "recorder.flush()", "X-Cache", "cache.DefaultTTL", "cacheWriteTimeout", "go func()"} {
		if !strings.Contains(cacheMiddleware, expected) {
			t.Fatalf("expected cache middleware to contain %q", expected)
		}
	}

	appBytes, err := os.ReadFile(filepath.Join(outputDir, "internal", "app", "app.go"))
	if err != nil {
		t.Fatalf("read app: %v", err)
	}
	appFile := string(appBytes)
	if !strings.Contains(appFile, "redisClient := cache.New(cfg.RedisAddr)") {
		t.Fatal("expected Redis initialization not to perform startup I/O")
	}
	if strings.Contains(appFile, "connect redis") {
		t.Fatal("expected Redis failure to degrade readiness instead of failing startup")
	}

	metricsBytes, err := os.ReadFile(filepath.Join(outputDir, "internal", "server", "metrics.go"))
	if err != nil {
		t.Fatalf("read metrics file: %v", err)
	}

	metrics := string(metricsBytes)
	for _, expected := range []string{"http_requests_total", "http_request_duration_seconds", "prometheus.NewRegistry()", "promhttp.HandlerFor"} {
		if !strings.Contains(metrics, expected) {
			t.Fatalf("expected metrics file to contain %q", expected)
		}
	}
	if strings.Contains(metrics, "promauto") {
		t.Fatal("expected metrics collectors not to use the global Prometheus registry")
	}

	routesBytes, err := os.ReadFile(filepath.Join(outputDir, "internal", "server", "routes.go"))
	if err != nil {
		t.Fatalf("read routes file: %v", err)
	}
	if !strings.Contains(string(routesBytes), "otelhttp.NewMiddleware") {
		t.Fatal("expected observability block to instrument HTTP requests")
	}

	for _, path := range []string{
		"internal/api/router.go",
		"internal/platform/database/postgres.go",
		"internal/platform/cache/redis.go",
		"internal/platform/o11y/o11y.go",
	} {
		if _, err := os.Stat(filepath.Join(outputDir, path)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be absent, got err=%v", path, err)
		}
	}

	dockerfileBytes, err := os.ReadFile(filepath.Join(outputDir, "deploy", "Dockerfile"))
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}

	dockerfile := string(dockerfileBytes)
	for _, expected := range []string{"FROM golang:" + detectGoVersion() + "-alpine AS builder", "HEALTHCHECK", "USER app", "go mod tidy"} {
		if !strings.Contains(dockerfile, expected) {
			t.Fatalf("expected Dockerfile to contain %q", expected)
		}
	}

	buildGeneratedProject(t, outputDir)
}

func buildGeneratedProject(t *testing.T, outputDir string) {
	t.Helper()

	tidyCommand := exec.Command("go", "mod", "tidy")
	tidyCommand.Dir = outputDir
	tidyCommand.Env = append(os.Environ(), "GOWORK=off")
	tidyOutput, err := tidyCommand.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, string(tidyOutput))
	}

	command := exec.Command("go", "build", "./...")
	command.Dir = outputDir
	command.Env = append(os.Environ(), "GOWORK=off")
	buildOutput, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, string(buildOutput))
	}

	testCommand := exec.Command("go", "test", "./...")
	testCommand.Dir = outputDir
	testCommand.Env = append(os.Environ(), "GOWORK=off")
	testOutput, err := testCommand.CombinedOutput()
	if err != nil {
		t.Fatalf("go test failed: %v\n%s", err, string(testOutput))
	}
}
