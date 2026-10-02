package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mexirica/chi-template/internal/cli"
	"github.com/mexirica/chi-template/internal/tui"
)

func main() {
	handled, err := cli.Run(os.Args[1:], os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cli failed: %v\n", err)
		os.Exit(1)
	}
	if handled {
		return
	}

	program := tea.NewProgram(tui.New(), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "bubble tea failed: %v\n", err)
		os.Exit(1)
	}
}
