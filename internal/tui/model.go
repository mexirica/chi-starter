package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mexirica/chi-starter/internal/blueprint"
)

type step int

const (
	stepProjectName step = iota
	stepModulePath
	stepOutputDir
	stepBlocks
	stepConfirm
	stepResult
)

type generatedMsg struct {
	result    blueprint.Result
	outputDir string
	err       error
}

type model struct {
	step        step
	nameInput   textinput.Model
	moduleInput textinput.Model
	outputInput textinput.Model
	blocks      []blueprint.Block
	selected    map[string]bool
	cursor      int
	width       int
	height      int
	generating  bool
	result      *blueprint.Result
	outputDir   string
	err         string
}

var (
	headlineStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	subtitleStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	mutedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("204")).Bold(true)
	successStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	panelStyle        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(1, 2)
	sectionTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("111"))
	keyLabelStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("24")).Padding(0, 1)
	stepActiveStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("86")).Padding(0, 1)
	stepDoneStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("120")).Padding(0, 1)
	stepPendingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Padding(0, 1)
)

func New() tea.Model {
	nameInput := textinput.New()
	nameInput.Placeholder = "my-service"
	nameInput.Focus()
	nameInput.Prompt = "> "

	moduleInput := textinput.New()
	moduleInput.Placeholder = "github.com/you/my-service"
	moduleInput.Prompt = "> "

	outputInput := textinput.New()
	outputInput.Placeholder = "../my-service"
	outputInput.Prompt = "> "

	selected := map[string]bool{}
	for _, block := range blueprint.OptionalBlocks() {
		selected[block.ID] = false
	}

	return model{
		step:        stepProjectName,
		nameInput:   nameInput,
		moduleInput: moduleInput,
		outputInput: outputInput,
		blocks:      blueprint.OptionalBlocks(),
		selected:    selected,
	}
}

func (model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case generatedMsg:
		m.generating = false
		m.outputDir = msg.outputDir
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}

		m.result = &msg.result
		m.err = ""
		m.step = stepResult
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	if m.generating {
		return m, nil
	}

	switch m.step {
	case stepProjectName:
		return m.updateProjectName(message)
	case stepModulePath:
		return m.updateModulePath(message)
	case stepOutputDir:
		return m.updateOutputDir(message)
	case stepBlocks:
		return m.updateBlocks(message)
	case stepConfirm:
		return m.updateConfirm(message)
	case stepResult:
		return m.updateResult(message)
	default:
		return m, nil
	}
}

func (m model) View() string {
	steps := []string{"Project", "Module", "Output", "Blocks", "Review", "Done"}
	header := strings.Join([]string{
		headlineStyle.Render("Chi Starter"),
		subtitleStyle.Render("Generate a clean, scalable Go backend baseline."),
		renderStepper(steps, m.step),
	}, "\n")

	var body string
	switch m.step {
	case stepProjectName:
		body = m.renderPanel(
			"Project name",
			m.nameInput.View(),
			"Choose a lowercase slug-like name (example: my-service).",
		)
	case stepModulePath:
		exampleName := sanitizeName(m.nameInput.Value())
		if exampleName == "" {
			exampleName = "my-service"
		}
		body = m.renderPanel(
			"Go module path",
			m.moduleInput.View(),
			"Example: github.com/you/"+exampleName,
		)
	case stepOutputDir:
		body = m.renderPanel(
			"Output directory",
			m.outputInput.View(),
			"Use an empty or non-existing directory.",
		)
	case stepBlocks:
		body = m.renderPanel("Select building blocks", m.blocksView(), "Tip: optional blocks shape the generated stack.")
	case stepConfirm:
		body = m.renderPanel("Review configuration", m.confirmView(), m.confirmHint())
	case stepResult:
		body = m.renderPanel("Generation result", m.resultView(), "Press Enter to exit.")
	}

	content := strings.Join([]string{header, "", body, "", m.footerView()}, "\n")
	if m.err != "" {
		content = content + "\n\n" + errorStyle.Render("Error: "+m.err)
	}

	if m.width > 0 {
		return lipgloss.NewStyle().Padding(1, 2).Width(m.width).Render(content)
	}

	return lipgloss.NewStyle().Padding(1, 2).Render(content)
}

func (m model) footerView() string {
	parts := []string{
		keyLabelStyle.Render("enter") + " continue",
		keyLabelStyle.Render("esc") + " back",
		keyLabelStyle.Render("ctrl+c") + " quit",
	}
	if m.step == stepBlocks {
		parts = append([]string{keyLabelStyle.Render("space") + " toggle block"}, parts...)
	}

	return mutedStyle.Render(strings.Join(parts, "   "))
}

func (m model) updateProjectName(message tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := message.(tea.KeyMsg); ok && key.String() == "enter" {
		name := sanitizeName(m.nameInput.Value())
		if name == "" {
			m.err = "project name is required"
			return m, nil
		}

		m.err = ""
		m.nameInput.SetValue(name)
		if strings.TrimSpace(m.moduleInput.Value()) == "" {
			m.moduleInput.SetValue("github.com/you/" + name)
		}
		if strings.TrimSpace(m.outputInput.Value()) == "" {
			m.outputInput.SetValue(filepath.Join("..", name))
		}
		m.nameInput.Blur()
		m.moduleInput.Focus()
		m.step = stepModulePath
		return m, nil
	}

	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(message)
	return m, cmd
}

func (m model) updateModulePath(message tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := message.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			if strings.TrimSpace(m.moduleInput.Value()) == "" {
				m.err = "module path is required"
				return m, nil
			}

			m.err = ""
			m.moduleInput.Blur()
			m.outputInput.Focus()
			m.step = stepOutputDir
			return m, nil
		case "esc":
			m.moduleInput.Blur()
			m.nameInput.Focus()
			m.step = stepProjectName
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.moduleInput, cmd = m.moduleInput.Update(message)
	return m, cmd
}

func (m model) updateOutputDir(message tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := message.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			if strings.TrimSpace(m.outputInput.Value()) == "" {
				m.err = "output directory is required"
				return m, nil
			}

			m.err = ""
			m.outputInput.Blur()
			m.step = stepBlocks
			return m, nil
		case "esc":
			m.outputInput.Blur()
			m.moduleInput.Focus()
			m.step = stepModulePath
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.outputInput, cmd = m.outputInput.Update(message)
	return m, cmd
}

func (m model) updateBlocks(message tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := message.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.blocks)-1 {
				m.cursor++
			}
		case " ":
			block := m.blocks[m.cursor]
			m.selected[block.ID] = !m.selected[block.ID]
		case "enter":
			m.step = stepConfirm
		case "esc":
			m.outputInput.Focus()
			m.step = stepOutputDir
		}
	}

	return m, nil
}

func (m model) updateConfirm(message tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := message.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			m.err = ""
			m.generating = true
			m.outputDir = m.outputInput.Value()
			return m, m.generate()
		case "esc":
			m.step = stepBlocks
			return m, nil
		}
	}

	return m, nil
}

func (m model) updateResult(message tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := message.(tea.KeyMsg); ok && key.String() == "enter" {
		return m, tea.Quit
	}

	return m, nil
}

func (m model) generate() tea.Cmd {
	options := blueprint.Options{
		ProjectName: m.nameInput.Value(),
		ModulePath:  m.moduleInput.Value(),
		OutputDir:   m.outputInput.Value(),
		Blocks:      m.selected,
	}

	return func() tea.Msg {
		result, err := blueprint.Generate(options)
		return generatedMsg{result: result, outputDir: options.OutputDir, err: err}
	}
}

func (m model) confirmView() string {
	rows := [][]string{
		{"Project", m.nameInput.Value()},
		{"Module", m.moduleInput.Value()},
		{"Output", m.outputInput.Value()},
		{"Blocks", strings.Join(m.selectedBlockIDs(), ", ")},
	}

	maxKey := 0
	for _, row := range rows {
		if len(row[0]) > maxKey {
			maxKey = len(row[0])
		}
	}

	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("%-*s : %s", maxKey, row[0], row[1]))
	}

	return strings.Join(lines, "\n")
}

func (m model) confirmHint() string {
	if m.generating {
		return "Generating files..."
	}

	return "Press Enter to generate or Esc to go back."
}

func (m model) resultView() string {
	if m.result == nil {
		return mutedStyle.Render("No files generated.")
	}

	preview, truncated := m.previewFiles(10)

	rows := []string{
		successStyle.Render("Project generated successfully."),
		fmt.Sprintf("Output: %s", m.outputDir),
		fmt.Sprintf("Files: %d", len(m.result.Files)),
		"",
		sectionTitleStyle.Render("Preview"),
		"- " + strings.Join(preview, "\n- "),
	}

	if truncated {
		rows = append(rows, mutedStyle.Render(fmt.Sprintf("... and %d more files", len(m.result.Files)-len(preview))))
	}

	rows = append(
		rows,
		"",
		sectionTitleStyle.Render("Next steps"),
		fmt.Sprintf("1. cd %s", m.outputDir),
		"2. go mod tidy",
		"3. make run",
	)

	return strings.Join(rows, "\n")
}

func (m model) blocksView() string {
	rows := []string{
		mutedStyle.Render("Core HTTP with Chi is always included."),
		mutedStyle.Render(fmt.Sprintf("Optional blocks selected: %d", m.selectedBlockCount())),
		"",
	}

	for index, block := range m.blocks {
		cursor := " "
		if m.cursor == index {
			cursor = ">"
		}

		mark := "[ ]"
		if m.selected[block.ID] {
			mark = "[x]"
		}

		name := block.Title
		desc := block.Description
		if m.selected[block.ID] {
			name = successStyle.Render(name)
		}

		rows = append(rows, fmt.Sprintf("%s %s %s", cursor, mark, name))
		rows = append(rows, "  "+mutedStyle.Render(desc))
	}

	return strings.Join(rows, "\n")
}

func (m model) selectedBlockIDs() []string {
	selected := []string{"core"}
	for _, block := range m.blocks {
		if m.selected[block.ID] {
			selected = append(selected, block.ID)
		}
	}

	return selected
}

func (m model) selectedBlockCount() int {
	count := 0
	for _, enabled := range m.selected {
		if enabled {
			count++
		}
	}

	return count
}

func (m model) renderPanel(title string, body string, footer string) string {
	parts := []string{
		sectionTitleStyle.Render(title),
		"",
		body,
	}
	if strings.TrimSpace(footer) != "" {
		parts = append(parts, "", mutedStyle.Render(footer))
	}

	panelContent := strings.Join(parts, "\n")
	if m.width > 0 {
		maxWidth := panelWidth(m.width)
		return panelStyle.MaxWidth(maxWidth).Render(panelContent)
	}

	return panelStyle.Render(panelContent)
}

func renderStepper(steps []string, current step) string {
	parts := make([]string, 0, len(steps))
	for index, item := range steps {
		switch {
		case index < int(current):
			parts = append(parts, stepDoneStyle.Render("x "+item))
		case index == int(current):
			parts = append(parts, stepActiveStyle.Render("> "+item))
		default:
			parts = append(parts, stepPendingStyle.Render("- "+item))
		}
	}

	return strings.Join(parts, " ")
}

func sanitizeName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.Trim(value, "-")
	return value
}

func sortedPreview(values []string) []string {
	cloned := append([]string(nil), values...)
	sort.Strings(cloned)
	return cloned
}

func (m model) previewFiles(limit int) ([]string, bool) {
	if m.result == nil || len(m.result.Files) == 0 {
		return nil, false
	}

	preview := sortedPreview(m.result.Files)
	if len(preview) <= limit {
		return preview, false
	}

	return preview[:limit], true
}

func panelWidth(screenWidth int) int {
	width := screenWidth - 6
	if width < 54 {
		return 54
	}
	if width > 110 {
		return 110
	}
	return width
}
