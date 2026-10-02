package blueprint

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Result struct {
	Files []string
}

func Generate(options Options) (Result, error) {
	if strings.TrimSpace(options.ProjectName) == "" {
		return Result{}, fmt.Errorf("project name is required")
	}

	if strings.TrimSpace(options.ModulePath) == "" {
		return Result{}, fmt.Errorf("module path is required")
	}

	if strings.TrimSpace(options.OutputDir) == "" {
		return Result{}, fmt.Errorf("output directory is required")
	}

	if err := ensureWritableOutput(options.OutputDir); err != nil {
		return Result{}, err
	}

	files, err := renderFiles(options)
	if err != nil {
		return Result{}, err
	}

	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	written := make([]string, 0, len(paths))
	for _, relativePath := range paths {
		absolutePath := filepath.Join(options.OutputDir, relativePath)
		if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
			return Result{}, fmt.Errorf("create directory for %s: %w", relativePath, err)
		}

		if err := os.WriteFile(absolutePath, []byte(files[relativePath]), 0o644); err != nil {
			return Result{}, fmt.Errorf("write %s: %w", relativePath, err)
		}

		written = append(written, relativePath)
	}

	return Result{Files: written}, nil
}

func ensureWritableOutput(outputDir string) error {
	entries, err := os.ReadDir(outputDir)
	if err == nil && len(entries) > 0 {
		return fmt.Errorf("output directory %q already exists and is not empty", outputDir)
	}

	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect output directory: %w", err)
	}

	return os.MkdirAll(outputDir, 0o755)
}
