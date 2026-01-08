package cmd

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/simonyos/Z-CODE/internal/agent"
	"github.com/simonyos/Z-CODE/internal/config"
	"github.com/simonyos/Z-CODE/internal/llm"
	"github.com/simonyos/Z-CODE/internal/tui"
)

var (
	providerFlag string
	modelFlag    string
)

var rootCmd = &cobra.Command{
	Use:   "zcode",
	Short: "AI coding assistant with interactive TUI",
	Long: `Z-Code is an AI-powered coding assistant that supports multiple LLM providers.
It features a beautiful terminal user interface with tool calling capabilities
for file operations and shell commands.

Supported providers:
  claude     - Claude Code CLI (default)
  gemini     - Gemini CLI
  openai     - OpenAI API (requires OPENAI_API_KEY)
  openrouter - OpenRouter API (requires OPENROUTER_API_KEY)
  litellm    - LiteLLM Proxy (unified interface to 100+ LLMs)`,
	Run: runChat,
}

func runChat(cmd *cobra.Command, args []string) {
	// Load config for defaults
	cfg := config.Get()

	// Use config defaults if flags not set
	selectedProvider := providerFlag
	if selectedProvider == "" && cfg.DefaultProvider != "" {
		selectedProvider = cfg.DefaultProvider
	}
	if selectedProvider == "" {
		selectedProvider = "claude"
	}

	selectedModel := modelFlag
	if selectedModel == "" && cfg.DefaultModel != "" {
		selectedModel = cfg.DefaultModel
	}

	// Create LLM provider based on selection
	var provider llm.Provider
	var modelName string

	switch strings.ToLower(selectedProvider) {
	case "openai":
		model := selectedModel
		if model == "" {
			model = "gpt-4o" // Default OpenAI model
		}
		provider = llm.NewOpenAI(model)
		modelName = model
	case "openrouter":
		model := selectedModel
		if model == "" {
			model = "anthropic/claude-sonnet-4" // Default OpenRouter model
		}
		provider = llm.NewOpenRouter(model)
		modelName = model
	case "gemini":
		provider = llm.NewGeminiCLI()
		modelName = "gemini"
	case "claude":
		provider = llm.NewClaudeCLI()
		modelName = "claude"
	case "litellm":
		model := selectedModel
		if model == "" {
			model = "gpt-4o" // Default LiteLLM model
		}
		provider = llm.NewLiteLLM(model)
		modelName = model
	default:
		fmt.Printf("Unknown provider: %s\n", selectedProvider)
		fmt.Println("Supported providers: claude, gemini, openai, openrouter, litellm")
		os.Exit(1)
	}

	// Create agent with confirmation function
	ag := agent.New(provider, tui.ConfirmAction)

	// Start TUI with options to prevent terminal query responses from appearing
	p := tea.NewProgram(
		tui.New(ag, modelName),
		tea.WithAltScreen(),
		tea.WithoutBracketedPaste(), // Disable bracketed paste to avoid escape sequence issues
	)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&providerFlag, "provider", "p", "", "LLM provider (claude, gemini, openai, openrouter, litellm)")
	rootCmd.Flags().StringVarP(&modelFlag, "model", "m", "", "Model to use (provider-specific)")
}
