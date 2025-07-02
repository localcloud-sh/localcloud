package templates

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/localcloud-sh/localcloud/internal/system"
)

// SetupWizard handles interactive template setup
type SetupWizard struct {
	template     Template
	systemCheck  SystemChecker
	portManager  *PortManager
	modelManager ModelManager
	generator    *Generator
}

// SystemChecker interface for system resource checking
type SystemChecker interface {
	GetSystemInfo() (system.SystemInfo, error)
	CheckRequirements(minRAM, minDisk int64) error
	RecommendModel(models []system.ModelSpec, availableRAM int64) *system.ModelSpec
}

// ModelManager interface for AI model management
type ModelManager interface {
	ListModels() ([]ModelInfo, error)
	PullModel(name string) error
	GetModelInfo(name string) (*ModelInfo, error)
}

// ModelInfo contains information about an AI model
type ModelInfo struct {
	Name     string
	Size     int64
	Modified string
}

// NewSetupWizard creates a new setup wizard
func NewSetupWizard(template Template, systemCheck SystemChecker, portManager *PortManager, modelManager ModelManager, generator *Generator) *SetupWizard {
	return &SetupWizard{
		template:     template,
		systemCheck:  systemCheck,
		portManager:  portManager,
		modelManager: modelManager,
		generator:    generator,
	}
}

// Run executes the setup wizard
func (w *SetupWizard) Run(ctx context.Context, templateName string, options SetupOptions) error {
	// Get template metadata
	metadata := w.template.GetMetadata()

	// Check system requirements
	sysInfo, err := w.systemCheck.GetSystemInfo()
	if err != nil {
		return fmt.Errorf("failed to check system: %w", err)
	}

	// Validate system resources
	if err := w.template.Validate(SystemResources{
		TotalRAM:            sysInfo.TotalRAM,
		AvailableRAM:        sysInfo.AvailableRAM,
		TotalDisk:           sysInfo.TotalDisk,
		AvailableDisk:       sysInfo.AvailableDisk,
		CPUCount:            sysInfo.CPUCount,
		DockerInstalled:     sysInfo.DockerInstalled,
		DockerVersion:       sysInfo.DockerVersion,
		OllamaInstalled:     sysInfo.OllamaInstalled,
		LocalLlamaInstalled: sysInfo.LocalLlamaInstalled,
		Platform:            sysInfo.Platform,
		Architecture:        sysInfo.Architecture,
	}); err != nil {
		return err
	}

	// Run interactive setup if not all options provided
	if !w.hasAllRequiredOptions(options) {
		// Initialize inputs
		inputs := make([]textinput.Model, 2)

		// Project name input
		projectInput := textinput.New()
		projectInput.Placeholder = templateName + "-app"
		projectInput.Focus()
		projectInput.CharLimit = 50
		projectInput.Width = 30
		projectInput.Prompt = "Project name: "
		inputs[0] = projectInput

		// Model selection input
		modelInput := textinput.New()
		modelInput.Placeholder = "qwen2.5:3b"
		modelInput.CharLimit = 50
		modelInput.Width = 30
		modelInput.Prompt = "AI Model: "
		inputs[1] = modelInput

		model := initialModel{
			wizard:       w,
			templateName: templateName,
			metadata:     metadata,
			sysInfo:      sysInfo,
			options:      options,
			inputs:       inputs,
			step:         0,
		}

		p := tea.NewProgram(model)
		finalModel, err := p.Run()
		if err != nil {
			return fmt.Errorf("setup wizard failed: %w", err)
		}

		// Extract options from final model
		if m, ok := finalModel.(completedModel); ok {
			options = m.options
		} else {
			return fmt.Errorf("setup cancelled")
		}
	}

	// Allocate ports
	ports, err := w.portManager.AllocateTemplatePorts(metadata.Services, options)
	if err != nil {
		return fmt.Errorf("failed to allocate ports: %w", err)
	}

	// Generate template variables
	vars := w.createTemplateVars(options, ports)

	// Generate project files
	if err := w.generator.Generate(templateName, options.ProjectName, vars); err != nil {
		return fmt.Errorf("failed to generate project: %w", err)
	}

	// Run template-specific generation
	if err := w.template.Generate(options.ProjectName, options); err != nil {
		return fmt.Errorf("template generation failed: %w", err)
	}

	// Start services unless skipped
	if !options.SkipDocker {
		if err := w.startServices(ctx, options.ProjectName, metadata.Services); err != nil {
			return fmt.Errorf("failed to start services: %w", err)
		}
	}

	// Run post-setup tasks
	if err := w.template.PostSetup(); err != nil {
		return fmt.Errorf("post-setup failed: %w", err)
	}

	// Print success message
	w.printSuccess(options.ProjectName, ports, options.ModelName)

	return nil
}

// hasAllRequiredOptions checks if all required options are provided
func (w *SetupWizard) hasAllRequiredOptions(options SetupOptions) bool {
	return options.ProjectName != "" && options.ModelName != ""
}

// createTemplateVars creates template variables from options and ports
func (w *SetupWizard) createTemplateVars(options SetupOptions, ports *PortAllocation) TemplateVars {
	// Generate secure passwords and secrets
	dbPassword := options.DatabasePassword
	if dbPassword == "" {
		dbPassword = generatePassword(16)
	}

	jwtSecret := generatePassword(32)

	return TemplateVars{
		ProjectName:      options.ProjectName,
		APIPort:          ports.API,
		FrontendPort:     ports.Frontend,
		DatabasePort:     ports.Database,
		CachePort:        ports.Cache,
		StoragePort:      ports.Storage,
		StorageUIPort:    ports.StorageUI,
		AIPort:           ports.AI,
		ModelName:        options.ModelName,
		DatabasePassword: dbPassword,
		DatabaseUser:     "localcloud",
		DatabaseName:     strings.ReplaceAll(options.ProjectName, "-", "_"),
		JWTSecret:        jwtSecret,
		APIBaseURL:       fmt.Sprintf("http://localhost:%d", ports.API),
		FrontendURL:      fmt.Sprintf("http://localhost:%d", ports.Frontend),
		Custom:           make(map[string]interface{}),
	}
}

// startServices starts Docker services
func (w *SetupWizard) startServices(ctx context.Context, projectPath string, services []string) error {
	// TODO: Integrate with existing Docker management
	// For now, just run docker-compose
	composePath := filepath.Join(projectPath, "docker-compose.yml")
	if _, err := os.Stat(composePath); err != nil {
		return fmt.Errorf("docker-compose.yml not found")
	}

	// This would integrate with the existing Docker manager
	// For now, return nil to indicate success
	return nil
}

// printSuccess prints success message with next steps
func (w *SetupWizard) printSuccess(projectName string, ports *PortAllocation, modelName string) {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("10"))

	fmt.Println()
	fmt.Println(style.Render("✓ Setup completed successfully!"))
	fmt.Println()
	fmt.Printf("Project created at: %s\n", projectName)
	fmt.Printf("AI Model: %s\n", modelName)
	fmt.Println()
	fmt.Println("Services:")
	if ports.Frontend > 0 {
		fmt.Printf("  Frontend: http://localhost:%d\n", ports.Frontend)
	}
	if ports.API > 0 {
		fmt.Printf("  API:      http://localhost:%d\n", ports.API)
	}
	if ports.Database > 0 {
		fmt.Printf("  Database: localhost:%d\n", ports.Database)
	}
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  cd %s\n", projectName)
	fmt.Println("  lc start")
	fmt.Println()
}

// generatePassword generates a random password
func generatePassword(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to simple password
		return "localcloud-" + fmt.Sprintf("%d", length)
	}
	return hex.EncodeToString(bytes)[:length]
}

// Bubble Tea models for interactive setup

type initialModel struct {
	wizard       *SetupWizard
	templateName string
	metadata     TemplateMetadata
	sysInfo      system.SystemInfo
	options      SetupOptions

	// UI state
	step    int
	inputs  []textinput.Model
	spinner spinner.Model
	err     error
}

type completedModel struct {
	options SetupOptions
}

// Implement tea.Model for initialModel

func (m initialModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m initialModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			// Process current step
			return m.nextStep()
		}
	}

	// Update current input
	if m.step < len(m.inputs) {
		var cmd tea.Cmd
		m.inputs[m.step], cmd = m.inputs[m.step].Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m initialModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	// Show current step
	return m.renderCurrentStep()
}

func (m initialModel) nextStep() (tea.Model, tea.Cmd) {
	// Process input based on current step
	switch m.step {
	case 0: // Project name
		m.options.ProjectName = m.inputs[0].Value()
		if m.options.ProjectName == "" {
			m.options.ProjectName = m.templateName + "-app"
		}
	case 1: // Model selection
		m.options.ModelName = m.inputs[1].Value()
	case 2: // API port
		// Parse and validate port
	}

	m.step++

	// Check if setup is complete
	if m.step >= len(m.inputs) {
		// Return completed model with tea.Quit
		return completedModel{options: m.options}, tea.Quit
	}

	return m, nil
}

func (m initialModel) renderCurrentStep() string {
	var b strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12"))

	b.WriteString(headerStyle.Render(fmt.Sprintf("Setting up %s\n\n", m.metadata.Name)))

	// System info
	if m.step == 0 {
		infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		b.WriteString(infoStyle.Render(fmt.Sprintf(
			"System: %s RAM available, %d CPU cores\n\n",
			system.FormatBytes(m.sysInfo.AvailableRAM),
			m.sysInfo.CPUCount,
		)))
	}

	// Current input
	if m.step < len(m.inputs) {
		b.WriteString(m.inputs[m.step].View())
	}

	// Help
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("Press Enter to continue, Esc to cancel"))

	return b.String()
}

// Implement tea.Model for completedModel

func (m completedModel) Init() tea.Cmd {
	return nil
}

func (m completedModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m completedModel) View() string {
	return ""
}
