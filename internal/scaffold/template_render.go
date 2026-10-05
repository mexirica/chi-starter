package scaffold

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"text/template"
)

var goVersionPattern = regexp.MustCompile(`^go([0-9]+\.[0-9]+(?:\.[0-9]+)?)`)

type templateData struct {
	ProjectName       string
	ModulePath        string
	GoVersion         string
	AppPort           string
	HasPostgres       bool
	HasRedis          bool
	HasObservability  bool
	HasDocker         bool
	SelectedBlockList []string
	GoRequirements    []string
}

func renderFiles(options Options) (map[string]string, error) {
	data := newTemplateData(options)

	files := map[string]string{}

	var err error
	files[".gitignore"], err = executeTemplate(gitignoreTemplate, data)
	if err != nil {
		return nil, err
	}

	files[".env.example"], err = executeTemplate(envTemplate, data)
	if err != nil {
		return nil, err
	}

	files[".air.toml"], err = executeTemplate(airTemplate, data)
	if err != nil {
		return nil, err
	}

	files["README.md"], err = executeTemplate(readmeTemplate, data)
	if err != nil {
		return nil, err
	}

	files["Makefile"], err = executeTemplate(makefileTemplate, data)
	if err != nil {
		return nil, err
	}

	files["go.mod"], err = executeTemplate(goModTemplate, data)
	if err != nil {
		return nil, err
	}

	files[filepath.ToSlash("cmd/api/main.go")], err = executeTemplate(mainTemplate, data)
	if err != nil {
		return nil, err
	}

	files[filepath.ToSlash("internal/app/app.go")], err = executeTemplate(appTemplate, data)
	if err != nil {
		return nil, err
	}

	files[filepath.ToSlash("internal/server/routes.go")], err = executeTemplate(routesTemplate, data)
	if err != nil {
		return nil, err
	}

	files[filepath.ToSlash("internal/server/health.go")], err = executeTemplate(healthTemplate, data)
	if err != nil {
		return nil, err
	}

	files[filepath.ToSlash("internal/server/middleware.go")], err = executeTemplate(middlewareTemplate, data)
	if err != nil {
		return nil, err
	}

	files[filepath.ToSlash("internal/server/metrics.go")], err = executeTemplate(metricsTemplate, data)
	if err != nil {
		return nil, err
	}

	files[filepath.ToSlash("internal/server/routes_test.go")], err = executeTemplate(routesTestTemplate, data)
	if err != nil {
		return nil, err
	}

	files[filepath.ToSlash("internal/server/server.go")], err = executeTemplate(serverTemplate, data)
	if err != nil {
		return nil, err
	}

	files[filepath.ToSlash("internal/config/config.go")], err = executeTemplate(configTemplate, data)
	if err != nil {
		return nil, err
	}

	files[filepath.ToSlash("internal/helpers/helpers.go")], err = executeTemplate(helpersTemplate, data)
	if err != nil {
		return nil, err
	}

	files[filepath.ToSlash("internal/logging/logging.go")], err = executeTemplate(loggingTemplate, data)
	if err != nil {
		return nil, err
	}

	files[filepath.ToSlash("internal/validation/validation.go")], err = executeTemplate(validationTemplate, data)
	if err != nil {
		return nil, err
	}

	if data.HasPostgres {
		files[filepath.ToSlash("internal/database/postgres.go")], err = executeTemplate(postgresTemplate, data)
		if err != nil {
			return nil, err
		}
	}

	if data.HasRedis {
		files[filepath.ToSlash("internal/cache/redis.go")], err = executeTemplate(redisTemplate, data)
		if err != nil {
			return nil, err
		}

		files[filepath.ToSlash("internal/middleware/cache.go")], err = executeTemplate(cacheMiddlewareTemplate, data)
		if err != nil {
			return nil, err
		}

		files[filepath.ToSlash("internal/middleware/cache_test.go")], err = executeTemplate(cacheMiddlewareTestTemplate, data)
		if err != nil {
			return nil, err
		}
	}

	if data.HasObservability {
		files[filepath.ToSlash("internal/o11y/o11y.go")], err = executeTemplate(o11yTemplate, data)
		if err != nil {
			return nil, err
		}
	}

	if data.HasDocker {
		files[filepath.ToSlash("deploy/Dockerfile")], err = executeTemplate(dockerfileTemplate, data)
		if err != nil {
			return nil, err
		}

		files[filepath.ToSlash("deploy/docker-compose.yml")], err = executeTemplate(dockerComposeTemplate, data)
		if err != nil {
			return nil, err
		}

		if data.HasObservability {
			files[filepath.ToSlash("deploy/otelcol/config.yaml")], err = executeTemplate(otelCollectorTemplate, data)
			if err != nil {
				return nil, err
			}

			files[filepath.ToSlash("deploy/prometheus/prometheus.yml")], err = executeTemplate(prometheusTemplate, data)
			if err != nil {
				return nil, err
			}

			files[filepath.ToSlash("deploy/loki/config.yaml")], err = executeTemplate(lokiTemplate, data)
			if err != nil {
				return nil, err
			}

			files[filepath.ToSlash("deploy/promtail/config.yaml")], err = executeTemplate(promtailTemplate, data)
			if err != nil {
				return nil, err
			}

			files[filepath.ToSlash("deploy/tempo/tempo-config.yaml")], err = executeTemplate(tempoTemplate, data)
			if err != nil {
				return nil, err
			}

			files[filepath.ToSlash("deploy/grafana/provisioning/datasources/datasource.yml")], err = executeTemplate(grafanaDatasourceTemplate, data)
			if err != nil {
				return nil, err
			}

			files[filepath.ToSlash("deploy/grafana/provisioning/dashboards/dashboard.yml")], err = executeTemplate(grafanaDashboardProviderTemplate, data)
			if err != nil {
				return nil, err
			}

			files[filepath.ToSlash("deploy/grafana/provisioning/dashboards/app_overview.json")], err = executeTemplate(grafanaAppDashboardTemplate, data)
			if err != nil {
				return nil, err
			}
		}
	}

	return files, nil
}

func newTemplateData(options Options) templateData {
	blocks := make([]string, 0, len(options.Blocks))
	for _, block := range OptionalBlocks() {
		if options.Has(block.ID) {
			blocks = append(blocks, block.Title)
		}
	}
	sort.Strings(blocks)

	requirements := []string{
		"github.com/go-chi/chi/v5 v5.1.0",
		"github.com/go-playground/validator/v10 v10.27.0",
	}
	if options.Has("redis") {
		requirements = append(requirements, "github.com/redis/go-redis/v9 v9.11.0")
	}
	if options.Has("postgres") {
		requirements = append(requirements, "github.com/jackc/pgx/v5 v5.7.5")
	}
	if options.Has("observability") {
		requirements = append(
			requirements,
			"github.com/prometheus/client_golang v1.20.3",
			"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.62.0",
			"go.opentelemetry.io/otel v1.37.0",
			"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.37.0",
			"go.opentelemetry.io/otel/sdk v1.37.0",
		)
	}
	sort.Strings(requirements)

	return templateData{
		ProjectName:       options.ProjectName,
		ModulePath:        options.ModulePath,
		GoVersion:         detectGoVersion(),
		AppPort:           "8080",
		HasPostgres:       options.Has("postgres"),
		HasRedis:          options.Has("redis"),
		HasObservability:  options.Has("observability"),
		HasDocker:         options.Has("docker"),
		SelectedBlockList: blocks,
		GoRequirements:    requirements,
	}
}

func executeTemplate(raw string, data templateData) (string, error) {
	tmpl, err := template.New("file").Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return strings.TrimLeft(buffer.String(), "\n"), nil
}

func detectGoVersion() string {
	output, err := exec.Command("go", "env", "GOVERSION").Output()
	if err == nil {
		if parsed := normalizeGoVersion(string(output)); parsed != "" {
			return parsed
		}
	}

	if parsed := normalizeGoVersion(runtime.Version()); parsed != "" {
		return parsed
	}

	return "1.26.1"
}

func normalizeGoVersion(raw string) string {
	trimmed := strings.TrimSpace(raw)
	matches := goVersionPattern.FindStringSubmatch(trimmed)
	if len(matches) != 2 {
		return ""
	}

	return matches[1]
}
