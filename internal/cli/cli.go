package cli

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mexirica/chi-starter/internal/blueprint"
)

func Run(args []string, stdout io.Writer, stderr io.Writer) (handled bool, err error) {
	if len(args) == 0 {
		return false, nil
	}

	switch args[0] {
	case "create", "new":
		return true, runCreate(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printRootUsage(stdout)
		return true, nil
	default:
		if strings.HasPrefix(args[0], "-") {
			return true, runCreate(args, stdout, stderr)
		}
		return false, nil
	}
}

func runCreate(args []string, stdout io.Writer, stderr io.Writer) error {
	flags := flag.NewFlagSet("create", flag.ContinueOnError)
	flags.SetOutput(stderr)

	name := flags.String("name", "", "Project name (required)")
	modulePath := flags.String("module", "", "Go module path (default: github.com/you/<name>)")
	outputDir := flags.String("output", "", "Output directory (default: ../<name>)")
	blocksCSV := flags.String("blocks", "", "Comma-separated blocks: postgres,redis,observability,docker")

	enablePostgres := flags.Bool("postgres", false, "Enable postgres block")
	enableRedis := flags.Bool("redis", false, "Enable redis block")
	enableObservability := flags.Bool("observability", false, "Enable observability block")
	enableDocker := flags.Bool("docker", false, "Enable docker block")

	flags.Usage = func() {
		printCreateUsage(stderr)
	}

	if err := flags.Parse(args); err != nil {
		return err
	}

	normalizedName := sanitizeName(*name)
	if normalizedName == "" {
		printCreateUsage(stderr)
		return fmt.Errorf("--name is required")
	}

	module := strings.TrimSpace(*modulePath)
	if module == "" {
		module = "github.com/you/" + normalizedName
	}

	output := strings.TrimSpace(*outputDir)
	if output == "" {
		output = filepath.Join("..", normalizedName)
	}

	blocks, err := parseBlocks(*blocksCSV)
	if err != nil {
		return err
	}

	blocks["postgres"] = blocks["postgres"] || *enablePostgres
	blocks["redis"] = blocks["redis"] || *enableRedis
	blocks["observability"] = blocks["observability"] || *enableObservability
	blocks["docker"] = blocks["docker"] || *enableDocker

	result, err := blueprint.Generate(blueprint.Options{
		ProjectName: normalizedName,
		ModulePath:  module,
		OutputDir:   output,
		Blocks:      blocks,
	})
	if err != nil {
		return err
	}

	selected := selectedBlocks(blocks)
	fmt.Fprintln(stdout, "Project generated successfully.")
	fmt.Fprintf(stdout, "Name: %s\n", normalizedName)
	fmt.Fprintf(stdout, "Module: %s\n", module)
	fmt.Fprintf(stdout, "Output: %s\n", output)
	fmt.Fprintf(stdout, "Blocks: %s\n", strings.Join(selected, ", "))
	fmt.Fprintf(stdout, "Files: %d\n\n", len(result.Files))

	fmt.Fprintln(stdout, "Next steps:")
	fmt.Fprintf(stdout, "1. cd %s\n", output)
	fmt.Fprintln(stdout, "2. go mod tidy")
	fmt.Fprintln(stdout, "3. make run")

	return nil
}

func parseBlocks(raw string) (map[string]bool, error) {
	result := map[string]bool{}
	if strings.TrimSpace(raw) == "" {
		return result, nil
	}

	available := map[string]bool{}
	for _, block := range blueprint.OptionalBlocks() {
		available[block.ID] = true
	}

	for _, part := range strings.Split(raw, ",") {
		name := strings.TrimSpace(strings.ToLower(part))
		if name == "" {
			continue
		}
		if !available[name] {
			return nil, fmt.Errorf("unknown block %q (available: postgres, redis, observability, docker)", name)
		}
		result[name] = true
	}

	return result, nil
}

func selectedBlocks(blocks map[string]bool) []string {
	selected := []string{"core"}
	for _, block := range blueprint.OptionalBlocks() {
		if blocks[block.ID] {
			selected = append(selected, block.ID)
		}
	}
	sort.Strings(selected[1:])
	return selected
}

func sanitizeName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.Trim(value, "-")
	return value
}

func printRootUsage(out io.Writer) {
	fmt.Fprintln(out, "Chi Starter")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  chi-starter               # interactive TUI")
	fmt.Fprintln(out, "  chi-starter create [flags]")
	fmt.Fprintln(out)
	printCreateUsage(out)
}

func printCreateUsage(out io.Writer) {
	fmt.Fprintln(out, "Create flags:")
	fmt.Fprintln(out, "  --name <name>             Project name (required)")
	fmt.Fprintln(out, "  --module <module>         Go module path")
	fmt.Fprintln(out, "  --output <dir>            Output directory")
	fmt.Fprintln(out, "  --blocks <list>           Comma-separated blocks")
	fmt.Fprintln(out, "  --postgres                Enable postgres block")
	fmt.Fprintln(out, "  --redis                   Enable redis block")
	fmt.Fprintln(out, "  --observability           Enable observability block")
	fmt.Fprintln(out, "  --docker                  Enable docker block")
}
