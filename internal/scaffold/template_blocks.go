package scaffold

const postgresTemplate = `
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}

	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}
`

const redisTemplate = `
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	DefaultTTL     = 5 * time.Minute
	DefaultTimeout = 2 * time.Second
)

func New(address string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         address,
		DialTimeout:  DefaultTimeout,
		ReadTimeout:  DefaultTimeout,
		WriteTimeout: DefaultTimeout,
	})
}

func Set(ctx context.Context, client *redis.Client, key string, value string, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = DefaultTTL
	}

	if err := client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("set cache value: %w", err)
	}

	return nil
}

func Get(ctx context.Context, client *redis.Client, key string) (string, error) {
	value, err := client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get cache value: %w", err)
	}

	return value, nil
}
`

const cacheMiddlewareTemplate = `
package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"{{ .ModulePath }}/internal/cache"
)

const cacheWriteTimeout = 500 * time.Millisecond

func Cache(ttl time.Duration, client *redis.Client) func(http.Handler) http.Handler {
	if ttl <= 0 {
		ttl = cache.DefaultTTL
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if client == nil || !isCacheableRequest(request) {
				next.ServeHTTP(writer, request)
				return
			}

			cacheKey := buildCacheKey(request)
			cachedValue, err := cache.Get(request.Context(), client, cacheKey)
			if err == nil && cachedValue != "" {
				writer.Header().Set("Content-Type", "application/json")
				writer.Header().Set("X-Cache", "hit")
				writer.WriteHeader(http.StatusOK)
				_, _ = writer.Write([]byte(cachedValue))
				return
			}

			recorder := newResponseRecorder(writer)
			next.ServeHTTP(recorder, request)
			recorder.Header().Set("X-Cache", "miss")
			recorder.flush()

			if recorder.statusCode != http.StatusOK || recorder.body.Len() == 0 || !isCacheableResponse(recorder.Header()) {
				return
			}

			payload := recorder.body.String()
			go func() {
				writeCtx, cancel := context.WithTimeout(context.Background(), cacheWriteTimeout)
				defer cancel()
				_ = cache.Set(writeCtx, client, cacheKey, payload, ttl)
			}()
		})
	}
}

func isCacheableRequest(request *http.Request) bool {
	return request.Method == http.MethodGet &&
		request.Header.Get("Authorization") == "" &&
		request.Header.Get("Cookie") == ""
}

func isCacheableResponse(header http.Header) bool {
	if header.Get("Set-Cookie") != "" || !strings.HasPrefix(header.Get("Content-Type"), "application/json") {
		return false
	}

	public := false
	for _, directive := range strings.Split(strings.ToLower(header.Get("Cache-Control")), ",") {
		switch strings.TrimSpace(directive) {
		case "public":
			public = true
		case "private", "no-store":
			return false
		}
	}

	return public
}

func buildCacheKey(request *http.Request) string {
	value := request.Method + ":" + request.URL.RequestURI() + ":" + request.Header.Get("Accept") + ":" + request.Header.Get("Accept-Encoding")
	checksum := sha256.Sum256([]byte(value))
	return "http-cache:" + hex.EncodeToString(checksum[:])
}

type responseRecorder struct {
	writer     http.ResponseWriter
	header     http.Header
	body       bytes.Buffer
	statusCode int
	wroteHeader bool
}

func newResponseRecorder(writer http.ResponseWriter) *responseRecorder {
	return &responseRecorder{writer: writer, header: make(http.Header), statusCode: http.StatusOK}
}

func (recorder *responseRecorder) Header() http.Header {
	return recorder.header
}

func (recorder *responseRecorder) WriteHeader(statusCode int) {
	if recorder.wroteHeader {
		return
	}

	recorder.statusCode = statusCode
	recorder.wroteHeader = true
}

func (recorder *responseRecorder) Write(payload []byte) (int, error) {
	if !recorder.wroteHeader {
		recorder.WriteHeader(http.StatusOK)
	}

	return recorder.body.Write(payload)
}

func (recorder *responseRecorder) flush() {
	for key, values := range recorder.header {
		recorder.writer.Header()[key] = append([]string(nil), values...)
	}
	recorder.writer.WriteHeader(recorder.statusCode)
	_, _ = recorder.writer.Write(recorder.body.Bytes())
}
`

const cacheMiddlewareTestTemplate = `
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsCacheableRequestRejectsCredentials(t *testing.T) {
	t.Parallel()

	for _, header := range []string{"Authorization", "Cookie"} {
		request := httptest.NewRequest(http.MethodGet, "/resource", nil)
		request.Header.Set(header, "sensitive")
		if isCacheableRequest(request) {
			t.Fatalf("expected request with %s not to be cacheable", header)
		}
	}
}

func TestIsCacheableResponseRequiresExplicitPublicJSON(t *testing.T) {
	t.Parallel()

	header := make(http.Header)
	header.Set("Content-Type", "application/json; charset=utf-8")
	if isCacheableResponse(header) {
		t.Fatal("expected response without public directive not to be cacheable")
	}

	header.Set("Cache-Control", "public, max-age=60")
	if !isCacheableResponse(header) {
		t.Fatal("expected explicitly public JSON response to be cacheable")
	}

	header.Set("Set-Cookie", "session=secret")
	if isCacheableResponse(header) {
		t.Fatal("expected response with Set-Cookie not to be cacheable")
	}
}

func TestResponseRecorderBuffersHeadersAndBody(t *testing.T) {
	t.Parallel()

	underlying := httptest.NewRecorder()
	recorder := newResponseRecorder(underlying)
	recorder.Header().Set("Content-Type", "application/json")
	_, _ = recorder.Write([]byte("payload"))

	if underlying.Body.Len() != 0 {
		t.Fatal("expected response to remain buffered before flush")
	}

	recorder.Header().Set("X-Cache", "miss")
	recorder.flush()
	if underlying.Header().Get("X-Cache") != "miss" {
		t.Fatal("expected X-Cache header to be flushed before the response")
	}
	if underlying.Body.String() != "payload" {
		t.Fatalf("expected buffered payload, got %q", underlying.Body.String())
	}
}
`

const helpersTemplate = `
package helpers

import (
	"encoding/json"
	"net/http"
)

func WriteJSON(writer http.ResponseWriter, statusCode int, payload any, headers ...http.Header) error {
	if len(headers) > 0 {
		for key, value := range headers[0] {
			writer.Header()[key] = value
		}
	}

	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(statusCode)

	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(true)
	return encoder.Encode(payload)
}

func ErrorJSON(writer http.ResponseWriter, err error, statusCode ...int) {
	code := http.StatusBadRequest
	if len(statusCode) > 0 {
		code = statusCode[0]
	}

	_ = WriteJSON(writer, code, map[string]any{
		"error":   true,
		"message": err.Error(),
	})
}
`

const validationTemplate = `
package validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/go-playground/validator/v10"
)

var (
	validate      = validator.New()
	ErrValidation = errors.New("validation failed")
)

type ErrorResponse struct {
	FailedField string ` + "`json:\"field\"`" + `
	Tag         string ` + "`json:\"tag\"`" + `
	Value       string ` + "`json:\"value,omitempty\"`" + `
}

func BindAndValidate(request *http.Request, destination any) ([]ErrorResponse, error) {
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		return nil, err
	}

	var extraPayload struct{}
	if err := decoder.Decode(&extraPayload); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("request body must contain a single JSON object")
		}
		return nil, err
	}

	err := validate.Struct(destination)
	if err == nil {
		return nil, nil
	}

	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return nil, err
	}

	errors := make([]ErrorResponse, 0, len(validationErrors))
	for _, validationError := range validationErrors {
		errors = append(errors, ErrorResponse{
			FailedField: validationError.Field(),
			Tag:         validationError.Tag(),
			Value:       validationError.Param(),
		})
	}

	return errors, ErrValidation
}
`

const loggingTemplate = `
package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

func New(environment string, level string) *slog.Logger {
	parsedLevel := parseLevel(level)
	options := &slog.HandlerOptions{Level: parsedLevel}

	if isProductionEnvironment(environment) {
		return slog.New(slog.NewJSONHandler(os.Stdout, options))
	}

	return slog.New(slog.NewTextHandler(io.Writer(os.Stdout), options))
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func isProductionEnvironment(environment string) bool {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "production", "prod":
		return true
	default:
		return false
	}
}
`

const o11yTemplate = `
package o11y

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type Config struct {
	ServiceName string
	Environment string
	Endpoint    string
}

func Init(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if cfg.Endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(cfg.Endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.DeploymentEnvironment(cfg.Environment),
		)),
	)

	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return provider.Shutdown, nil
}
`
