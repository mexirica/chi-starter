package blueprint

type Block struct {
	ID          string
	Title       string
	Description string
}

type Options struct {
	ProjectName string
	ModulePath  string
	OutputDir   string
	Blocks      map[string]bool
}

func (o Options) Has(blockID string) bool {
	if o.Blocks == nil {
		return false
	}

	return o.Blocks[blockID]
}

func OptionalBlocks() []Block {
	return []Block{
		{
			ID:          "postgres",
			Title:       "Database",
			Description: "Postgres + pgxpool scaffold.",
		},
		{
			ID:          "redis",
			Title:       "Redis",
			Description: "Redis client and readiness probe wiring.",
		},
		{
			ID:          "observability",
			Title:       "Observability",
			Description: "OTEL + Prometheus + Loki + Tempo + provisioned Grafana stack.",
		},
		{
			ID:          "docker",
			Title:       "Docker",
			Description: "Dockerfile and docker-compose with selected services.",
		},
	}
}
