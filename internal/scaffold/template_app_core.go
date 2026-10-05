package scaffold

const mainTemplate = `
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"{{ .ModulePath }}/internal/app"
	"{{ .ModulePath }}/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	application, err := app.New(ctx, cfg)
	if err != nil {
		log.Fatalf("create application: %v", err)
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- application.Run()
	}()

	select {
	case <-ctx.Done():
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("serve http: %v", err)
		}
	}

	if err := application.Shutdown(nil); err != nil {
		log.Fatalf("shutdown application: %v", err)
	}
}
`

const appTemplate = `
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"{{ .ModulePath }}/internal/config"
	"{{ .ModulePath }}/internal/logging"
	"{{ .ModulePath }}/internal/server"
{{- if .HasPostgres }}
	"github.com/jackc/pgx/v5/pgxpool"
	"{{ .ModulePath }}/internal/database"
{{- end }}
{{- if .HasRedis }}
	"github.com/redis/go-redis/v9"
	"{{ .ModulePath }}/internal/cache"
{{- end }}
{{- if .HasObservability }}
	"{{ .ModulePath }}/internal/o11y"
{{- end }}
)

type App struct {
	server          *http.Server
	logger          *slog.Logger
	shutdownTimeout time.Duration
{{- if .HasPostgres }}
	dbConn *pgxpool.Pool
{{- end }}
{{- if .HasRedis }}
	redisClient *redis.Client
{{- end }}
{{- if .HasObservability }}
	shutdownTelemetry func(context.Context) error
{{- end }}
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	logger := logging.New(cfg.AppEnv, cfg.LogLevel)
	slog.SetDefault(logger)

	application := &App{
		logger:          logger,
		shutdownTimeout: cfg.ShutdownTimeout,
	}
	checks := map[string]server.Probe{}
{{- if .HasObservability }}
	shutdownTelemetry, err := o11y.Init(ctx, o11y.Config{
		ServiceName: "{{ .ProjectName }}",
		Environment: cfg.AppEnv,
		Endpoint:    cfg.OTELExporterOTLPEndpoint,
	})
	if err != nil {
		return nil, fmt.Errorf("init observability: %w", err)
	}
	application.shutdownTelemetry = shutdownTelemetry
{{- end }}
{{- if .HasPostgres }}
	dbConn, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	application.dbConn = dbConn
	checks["postgres"] = application.dbConn.Ping
{{- end }}
{{- if .HasRedis }}
	redisClient := cache.New(cfg.RedisAddr)
	application.redisClient = redisClient
	checks["redis"] = func(ctx context.Context) error {
		return application.redisClient.Ping(ctx).Err()
	}
{{- end }}

	handler := server.NewRouter(server.RouterOptions{
		Logger:         logger,
		Checks:         checks,
		EnableMetrics:  {{ .HasObservability }},
		RequestTimeout: cfg.HTTPRequestTimeout,
	})
	application.server = server.New(server.Options{
		Port:              cfg.AppPort,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}, handler)

	return application, nil
}

func (application *App) Run() error {
	application.logger.Info("starting http server", "addr", application.server.Addr)
	if err := application.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

func (application *App) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if _, hasDeadline := ctx.Deadline(); !hasDeadline && application.shutdownTimeout > 0 {
		shutdownCtx, cancel := context.WithTimeout(ctx, application.shutdownTimeout)
		defer cancel()
		ctx = shutdownCtx
	}

	application.logger.Info("shutting down application")

	var serverErr error
	if application.server != nil {
		serverErr = application.server.Shutdown(ctx)
	}

	resourceErr := application.closeResources(ctx)
	return errors.Join(serverErr, resourceErr)
}

func (application *App) closeResources(ctx context.Context) error {
	var shutdownErr error
{{- if .HasObservability }}
	if application.shutdownTelemetry != nil {
		shutdownErr = errors.Join(shutdownErr, application.shutdownTelemetry(ctx))
	}
{{- end }}
{{- if .HasRedis }}
	if application.redisClient != nil {
		shutdownErr = errors.Join(shutdownErr, application.redisClient.Close())
	}
{{- end }}
{{- if .HasPostgres }}
	if application.dbConn != nil {
		application.dbConn.Close()
	}
{{- end }}

	return shutdownErr
}
`

const routesTemplate = `
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
{{- if .HasObservability }}
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
{{- end }}
)

type Probe func(context.Context) error

type RouterOptions struct {
	Logger         *slog.Logger
	Checks         map[string]Probe
	EnableMetrics  bool
	RequestTimeout time.Duration
}

func NewRouter(options RouterOptions) http.Handler {
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}

	requestTimeout := options.RequestTimeout
	if requestTimeout <= 0 {
		requestTimeout = 30 * time.Second
	}

	health := &healthHandler{checks: cloneChecks(options.Checks), logger: logger}
	metrics, metricsEndpoint := newHTTPMetrics(options.EnableMetrics)

	router := chi.NewRouter()
{{- if .HasObservability }}
	router.Use(otelhttp.NewMiddleware("{{ .ProjectName }}"))
{{- end }}
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Heartbeat("/ping"))
	router.Use(middleware.Timeout(requestTimeout))
	router.Use(requestLogger(logger, metrics))
	router.Get("/healthz", health.live)
	router.Get("/readyz", health.ready)
	router.Route("/v1", func(versioned chi.Router) {
		versioned.Get("/healthz", health.live)
		versioned.Get("/readyz", health.ready)
	})

	if metricsEndpoint != nil {
		router.Handle("/metrics", metricsEndpoint)
	}

	return router
}

func cloneChecks(checks map[string]Probe) map[string]Probe {
	if len(checks) == 0 {
		return map[string]Probe{}
	}

	clonedChecks := make(map[string]Probe, len(checks))
	for name, probe := range checks {
		clonedChecks[name] = probe
	}

	return clonedChecks
}
`

const healthTemplate = `
package server

import (
	"log/slog"
	"net/http"
	"sort"

	"{{ .ModulePath }}/internal/helpers"
)

type healthHandler struct {
	checks map[string]Probe
	logger *slog.Logger
}

func (handler *healthHandler) live(writer http.ResponseWriter, _ *http.Request) {
	_ = helpers.WriteJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
}

func (handler *healthHandler) ready(writer http.ResponseWriter, request *http.Request) {
	result := map[string]any{"status": "ready"}
	names := sortedProbeNames(handler.checks)

	degraded := false
	for _, name := range names {
		probe := handler.checks[name]
		if err := probe(request.Context()); err != nil {
			degraded = true
			handler.logger.Error("readiness check failed", "dependency", name, "error", err)
			result[name] = "unavailable"
			continue
		}

		result[name] = "ok"
	}

	if degraded {
		result["status"] = "degraded"
		_ = helpers.WriteJSON(writer, http.StatusServiceUnavailable, result)
		return
	}

	_ = helpers.WriteJSON(writer, http.StatusOK, result)
}

func sortedProbeNames(checks map[string]Probe) []string {
	names := make([]string, 0, len(checks))
	for name := range checks {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
`

const middlewareTemplate = `
package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func requestLogger(logger *slog.Logger, metrics httpMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			start := time.Now()
			recorder := &statusRecorder{ResponseWriter: writer, statusCode: http.StatusOK}

			next.ServeHTTP(recorder, request)

			route := routePattern(request)
			duration := time.Since(start)
			metrics.Observe(request.Method, route, recorder.statusCode, duration)

			attributes := []any{
				"method", request.Method,
				"path", request.URL.Path,
				"route", route,
				"status", recorder.statusCode,
				"duration_ms", duration.Milliseconds(),
				"bytes", recorder.bytesWritten,
				"request_id", middleware.GetReqID(request.Context()),
				"remote_ip", request.RemoteAddr,
			}

			if recorder.statusCode >= http.StatusInternalServerError {
				logger.Error("http request", attributes...)
				return
			}

			if recorder.statusCode >= http.StatusBadRequest {
				logger.Warn("http request", attributes...)
				return
			}

			logger.Info("http request", attributes...)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (recorder *statusRecorder) WriteHeader(statusCode int) {
	recorder.statusCode = statusCode
	recorder.ResponseWriter.WriteHeader(statusCode)
}

func (recorder *statusRecorder) Write(payload []byte) (int, error) {
	if recorder.statusCode == 0 {
		recorder.statusCode = http.StatusOK
	}

	written, err := recorder.ResponseWriter.Write(payload)
	recorder.bytesWritten += written
	return written, err
}

func routePattern(request *http.Request) string {
	if routeContext := chi.RouteContext(request.Context()); routeContext != nil {
		if pattern := routeContext.RoutePattern(); pattern != "" {
			return pattern
		}
	}

	return "unmatched"
}
`

const metricsTemplate = `
package server

import (
	"net/http"
	"time"
{{- if .HasObservability }}
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
{{- end }}
)

type httpMetrics interface {
	Observe(method string, route string, statusCode int, duration time.Duration)
}

type noopHTTPMetrics struct{}

func (noopHTTPMetrics) Observe(string, string, int, time.Duration) {}

func newHTTPMetrics(enabled bool) (httpMetrics, http.Handler) {
	if !enabled {
		return noopHTTPMetrics{}, nil
	}
{{- if .HasObservability }}
	registry := prometheus.NewRegistry()
	requestsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of handled HTTP requests.",
	}, []string{"method", "route", "status"})
	requestDurationSeconds := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Latency distribution of HTTP requests.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route", "status"})
	registry.MustRegister(requestsTotal, requestDurationSeconds)

	metrics := promHTTPMetrics{
		requestsTotal:          requestsTotal,
		requestDurationSeconds: requestDurationSeconds,
	}
	return metrics, promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
{{- else }}
	return noopHTTPMetrics{}, nil
{{- end }}
}

{{- if .HasObservability }}
type promHTTPMetrics struct {
	requestsTotal          *prometheus.CounterVec
	requestDurationSeconds *prometheus.HistogramVec
}

func (metrics promHTTPMetrics) Observe(method string, route string, statusCode int, duration time.Duration) {
	status := strconv.Itoa(statusCode)
	metrics.requestsTotal.WithLabelValues(method, route, status).Inc()
	metrics.requestDurationSeconds.WithLabelValues(method, route, status).Observe(duration.Seconds())
}
{{- end }}
`

const routesTestTemplate = `
package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestNewRouterHealthz(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()

	NewRouter(RouterOptions{}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
}

func TestNewRouterVersionedHealthz(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/v1/healthz", nil)
	recorder := httptest.NewRecorder()

	NewRouter(RouterOptions{}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
}

func TestNewRouterReadyz(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	recorder := httptest.NewRecorder()

	checks := map[string]Probe{
		"database": func(context.Context) error { return nil },
	}

	NewRouter(RouterOptions{Checks: checks}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
}

func TestNewRouterReadyzDegraded(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	recorder := httptest.NewRecorder()

	checks := map[string]Probe{
		"database": func(context.Context) error { return errors.New("db down") },
	}

	NewRouter(RouterOptions{Checks: checks}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "db down") {
		t.Fatalf("expected readiness response to hide dependency error, got %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), ` + "`\"database\":\"unavailable\"`" + `) {
		t.Fatalf("expected sanitized dependency status, got %s", recorder.Body.String())
	}
}

func TestNewRouterHandlesConcurrentRequests(t *testing.T) {
	t.Parallel()

	router := NewRouter(RouterOptions{})
	statuses := make(chan int, 32)
	var group sync.WaitGroup
	for range 32 {
		group.Add(1)
		go func() {
			defer group.Done()
			request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			statuses <- recorder.Code
		}()
	}

	group.Wait()
	close(statuses)
	for status := range statuses {
		if status != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, status)
		}
	}
}

{{- if .HasObservability }}
func TestNewRouterUsesIsolatedMetricsRegistries(t *testing.T) {
	t.Parallel()

	for range 2 {
		request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		recorder := httptest.NewRecorder()
		NewRouter(RouterOptions{EnableMetrics: true}).ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected metrics status %d, got %d", http.StatusOK, recorder.Code)
		}
	}
}
{{- end }}
`

const serverTemplate = `
package server

import (
	"net/http"
	"strings"
	"time"
)

type Options struct {
	Port              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

func New(options Options, handler http.Handler) *http.Server {
	if handler == nil {
		handler = http.NewServeMux()
	}

	readHeaderTimeout := options.ReadHeaderTimeout
	if readHeaderTimeout <= 0 {
		readHeaderTimeout = 5 * time.Second
	}

	readTimeout := options.ReadTimeout
	if readTimeout <= 0 {
		readTimeout = 10 * time.Second
	}

	writeTimeout := options.WriteTimeout
	if writeTimeout <= 0 {
		writeTimeout = 15 * time.Second
	}

	idleTimeout := options.IdleTimeout
	if idleTimeout <= 0 {
		idleTimeout = 60 * time.Second
	}

	return &http.Server{
		Addr:              normalizeAddress(options.Port),
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

func normalizeAddress(port string) string {
	trimmed := strings.TrimSpace(port)
	if trimmed == "" {
		return ":8080"
	}

	if strings.HasPrefix(trimmed, ":") {
		return trimmed
	}

	return ":" + trimmed
}
`

const configTemplate = `
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	AppPort                string
	AppEnv                 string
	LogLevel               string
	HTTPRequestTimeout     time.Duration
	HTTPReadHeaderTimeout  time.Duration
	HTTPReadTimeout        time.Duration
	HTTPWriteTimeout       time.Duration
	HTTPIdleTimeout        time.Duration
	ShutdownTimeout        time.Duration
{{- if .HasPostgres }}
	DatabaseURL            string
{{- end }}
{{- if .HasRedis }}
	RedisAddr              string
{{- end }}
{{- if .HasObservability }}
	OTELExporterOTLPEndpoint string
{{- end }}
}

func Load() (Config, error) {
	httpRequestTimeout, err := durationOrDefault("HTTP_REQUEST_TIMEOUT", "30s")
	if err != nil {
		return Config{}, err
	}

	httpReadHeaderTimeout, err := durationOrDefault("HTTP_READ_HEADER_TIMEOUT", "5s")
	if err != nil {
		return Config{}, err
	}

	httpReadTimeout, err := durationOrDefault("HTTP_READ_TIMEOUT", "10s")
	if err != nil {
		return Config{}, err
	}

	httpWriteTimeout, err := durationOrDefault("HTTP_WRITE_TIMEOUT", "15s")
	if err != nil {
		return Config{}, err
	}

	httpIdleTimeout, err := durationOrDefault("HTTP_IDLE_TIMEOUT", "60s")
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := durationOrDefault("APP_SHUTDOWN_TIMEOUT", "10s")
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppPort:               valueOrDefault("APP_PORT", "{{ .AppPort }}"),
		AppEnv:                valueOrDefault("APP_ENV", "development"),
		LogLevel:              valueOrDefault("LOG_LEVEL", "info"),
		HTTPRequestTimeout:    httpRequestTimeout,
		HTTPReadHeaderTimeout: httpReadHeaderTimeout,
		HTTPReadTimeout:       httpReadTimeout,
		HTTPWriteTimeout:      httpWriteTimeout,
		HTTPIdleTimeout:       httpIdleTimeout,
		ShutdownTimeout:       shutdownTimeout,
{{- if .HasPostgres }}
		DatabaseURL: valueOrDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/{{ .ProjectName }}?sslmode=disable"),
{{- end }}
{{- if .HasRedis }}
		RedisAddr: valueOrDefault("REDIS_ADDR", "localhost:6379"),
{{- end }}
{{- if .HasObservability }}
		OTELExporterOTLPEndpoint: os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
{{- end }}
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.AppPort) == "" {
		return fmt.Errorf("APP_PORT is required")
	}

	if strings.TrimSpace(cfg.AppEnv) == "" {
		return fmt.Errorf("APP_ENV is required")
	}

	if !isSupportedLogLevel(cfg.LogLevel) {
		return fmt.Errorf("LOG_LEVEL must be one of: debug, info, warn, error")
	}

	if cfg.HTTPRequestTimeout <= 0 {
		return fmt.Errorf("HTTP_REQUEST_TIMEOUT must be greater than zero")
	}
	if cfg.HTTPReadHeaderTimeout <= 0 {
		return fmt.Errorf("HTTP_READ_HEADER_TIMEOUT must be greater than zero")
	}
	if cfg.HTTPReadTimeout <= 0 {
		return fmt.Errorf("HTTP_READ_TIMEOUT must be greater than zero")
	}
	if cfg.HTTPWriteTimeout <= 0 {
		return fmt.Errorf("HTTP_WRITE_TIMEOUT must be greater than zero")
	}
	if cfg.HTTPIdleTimeout <= 0 {
		return fmt.Errorf("HTTP_IDLE_TIMEOUT must be greater than zero")
	}
	if cfg.ShutdownTimeout <= 0 {
		return fmt.Errorf("APP_SHUTDOWN_TIMEOUT must be greater than zero")
	}
{{- if .HasPostgres }}
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
{{- end }}
{{- if .HasRedis }}
	if strings.TrimSpace(cfg.RedisAddr) == "" {
		return fmt.Errorf("REDIS_ADDR is required")
	}
{{- end }}

	return nil
}

func valueOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func durationOrDefault(key string, fallback string) (time.Duration, error) {
	value := valueOrDefault(key, fallback)
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", key, err)
	}

	return parsed, nil
}

func isSupportedLogLevel(level string) bool {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}
`
