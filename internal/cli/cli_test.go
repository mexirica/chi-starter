package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunNoArgsFallsBackToTUI(t *testing.T) {
	t.Parallel()

	handled, err := Run(nil, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if handled {
		t.Fatal("expected no-args execution to fallback to TUI")
	}
}

func TestRunUnknownCommandFallsBackToTUI(t *testing.T) {
	t.Parallel()

	handled, err := Run([]string{"unknown"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if handled {
		t.Fatal("expected unknown command to fallback to TUI")
	}
}

func TestRunHelpHandled(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	handled, err := Run([]string{"help"}, stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !handled {
		t.Fatal("expected help to be handled")
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatal("expected help output to contain usage")
	}
}

func TestRunCreateWithDefaults(t *testing.T) {
	t.Parallel()

	outputDir := filepath.Join(t.TempDir(), "my-project")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	handled, err := Run([]string{"create", "--name", "my-project", "--output", outputDir}, stdout, stderr)
	if err != nil {
		t.Fatalf("expected no error, got %v, stderr=%s", err, stderr.String())
	}
	if !handled {
		t.Fatal("expected create command to be handled")
	}

	goModBytes, err := os.ReadFile(filepath.Join(outputDir, "go.mod"))
	if err != nil {
		t.Fatalf("expected generated go.mod, got %v", err)
	}
	goMod := string(goModBytes)
	if !strings.Contains(goMod, "module github.com/you/my-project") {
		t.Fatal("expected default module path based on project name")
	}

	if !strings.Contains(stdout.String(), "Project generated successfully.") {
		t.Fatal("expected success output for create command")
	}
}

func TestRunCreateWithInvalidBlock(t *testing.T) {
	t.Parallel()

	handled, err := Run([]string{"create", "--name", "my-project", "--blocks", "invalid"}, &bytes.Buffer{}, &bytes.Buffer{})
	if !handled {
		t.Fatal("expected create command to be handled")
	}
	if err == nil {
		t.Fatal("expected error for invalid block")
	}
}
