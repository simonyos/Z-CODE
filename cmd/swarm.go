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
	"github.com/simonyos/Z-CODE/internal/swarm"
	"github.com/simonyos/Z-CODE/internal/tools"
	"github.com/simonyos/Z-CODE/internal/tui"
)

var (
	swarmRoleFlag string
	swarmNameFlag string
	swarmRepoFlag string
)

var swarmCmd = &cobra.Command{
	Use:   "swarm",
	Short: "Multi-agent collaboration mode",
	Long: `HiveMind Swarm Mode enables multiple Z-CODE instances to collaborate.

Create a room as orchestrator or join an existing room with a specific role.
Agents communicate via NATS messaging and can delegate tasks to each other.

Available roles:
  ORCH    - Orchestrator: coordinates agents and makes decisions
  SA      - Solution Architect: designs systems and schemas
  BE_DEV  - Backend Developer: implements APIs and services
  FE_DEV  - Frontend Developer: implements UI components
  QA      - Quality Assurance: reviews code and tests
  DEVOPS  - DevOps Engineer: CI/CD and infrastructure
  DBA     - Database Administrator: schema and optimization
  SEC     - Security Engineer: security review and auth design
  HUMAN   - Human Observer: can message any agent`,
}

var swarmCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new swarm room as orchestrator",
	Long: `Create a new swarm room and join as the Orchestrator.

Other agents can join using the room code that will be displayed.

Examples:
  zcode swarm create
  zcode swarm create my-project
  zcode swarm create auth-feature --repo https://github.com/user/repo`,
	Args: cobra.MaximumNArgs(1),
	Run:  runSwarmCreate,
}

var swarmJoinCmd = &cobra.Command{
	Use:   "join <room-code> --role <ROLE>",
	Short: "Join an existing swarm room",
	Long: `Join an existing swarm room with a specific role.

You need the room code from the orchestrator who created the room.

Examples:
  zcode swarm join merry-panda-9k2j --role SA
  zcode swarm join merry-panda-9k2j --role BE_DEV
  zcode swarm join merry-panda-9k2j --role HUMAN`,
	Args: cobra.ExactArgs(1),
	Run:  runSwarmJoin,
}

var swarmListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available roles",
	Run:   runSwarmList,
}

func runSwarmCreate(cmd *cobra.Command, args []string) {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	// Load swarm config
	swarmConfig, err := swarm.LoadConfig()
	if err != nil {
		fmt.Printf("Warning: Could not load swarm config: %v\n", err)
		swarmConfig = swarm.DefaultSwarmConfig()
	}

	// Create swarm client
	client := swarm.NewClient(swarmConfig)

	// Create room config
	roomConfig := swarm.DefaultRoomConfig()
	if swarmRepoFlag != "" {
		roomConfig.ProjectRepo = swarmRepoFlag
	}

	// Create room in raw mode (TUI will handle message/presence channels directly)
	room, err := client.CreateRoomRaw(name, roomConfig)
	if err != nil {
		fmt.Printf("Error creating room: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Room created: %s\n", room.Name)
	fmt.Printf("Room code: %s\n", room.Code)
	fmt.Println("Share this code with other agents to join.")
	fmt.Println()

	// Start TUI in swarm mode
	startSwarmTUI(client, swarm.RoleOrchestrator)
}

func runSwarmJoin(cmd *cobra.Command, args []string) {
	roomCode := args[0]

	if swarmRoleFlag == "" {
		fmt.Println("Error: --role flag is required")
		fmt.Println("Available roles: ORCH, SA, BE_DEV, FE_DEV, QA, DEVOPS, DBA, SEC, HUMAN")
		os.Exit(1)
	}

	// Parse role
	role, err := swarm.ParseRole(strings.ToUpper(swarmRoleFlag))
	if err != nil {
		fmt.Printf("Error: Invalid role '%s'\n", swarmRoleFlag)
		fmt.Println("Available roles: ORCH, SA, BE_DEV, FE_DEV, QA, DEVOPS, DBA, SEC, HUMAN")
		os.Exit(1)
	}

	// Load swarm config
	swarmConfig, err := swarm.LoadConfig()
	if err != nil {
		fmt.Printf("Warning: Could not load swarm config: %v\n", err)
		swarmConfig = swarm.DefaultSwarmConfig()
	}

	// Create swarm client
	client := swarm.NewClient(swarmConfig)

	// Join room in raw mode (TUI will handle message/presence channels directly)
	if err := client.JoinRoomRaw(roomCode, role); err != nil {
		fmt.Printf("Error joining room: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Joined room: %s as %s\n", roomCode, role)

	// Start TUI in swarm mode
	startSwarmTUI(client, role)
}

func runSwarmList(cmd *cobra.Command, args []string) {
	roles := swarm.DefaultRoles()

	fmt.Println("Available Swarm Roles:")
	fmt.Println()

	for _, role := range swarm.AllRoles() {
		def := roles[role]
		if def != nil {
			fmt.Printf("  %-8s - %s\n", role, def.Name)
			fmt.Printf("           %s\n", def.Description)
			fmt.Println()
		}
	}
}

func startSwarmTUI(client *swarm.Client, role swarm.Role) {
	// Load config for LLM settings
	cfg := config.Get()

	// Check for role-specific LLM config
	swarmConfig, _ := swarm.LoadConfig()
	roleConfig := swarmConfig.GetRoleConfig(role)

	// Use config defaults if flags not set
	selectedProvider := providerFlag
	if selectedProvider == "" && roleConfig.LLMProvider != "" {
		selectedProvider = roleConfig.LLMProvider
	}
	if selectedProvider == "" && cfg.DefaultProvider != "" {
		selectedProvider = cfg.DefaultProvider
	}
	if selectedProvider == "" {
		selectedProvider = "claude"
	}

	selectedModel := modelFlag
	if selectedModel == "" && roleConfig.LLMModel != "" {
		selectedModel = roleConfig.LLMModel
	}
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
			model = "gpt-4o"
		}
		provider = llm.NewOpenAI(model)
		modelName = model
	case "openrouter":
		model := selectedModel
		if model == "" {
			model = "anthropic/claude-sonnet-4"
		}
		provider = llm.NewOpenRouter(model)
		modelName = model
	case "gemini":
		// Gemini is now accessed through LiteLLM or OpenRouter
		model := selectedModel
		if model == "" {
			model = "google/gemini-flash-1.5"
		}
		provider = llm.NewLiteLLM(model)
		modelName = model
	case "claude":
		// Claude is now accessed through LiteLLM or OpenRouter
		model := selectedModel
		if model == "" {
			model = "anthropic/claude-3.5-sonnet"
		}
		provider = llm.NewLiteLLM(model)
		modelName = model
	case "litellm":
		model := selectedModel
		if model == "" {
			model = "gpt-4o"
		}
		provider = llm.NewLiteLLM(model)
		modelName = model
	default:
		fmt.Printf("Unknown provider: %s\n", selectedProvider)
		os.Exit(1)
	}

	// Create agent with swarm-enhanced system prompt
	ag := agent.New(provider, tui.ConfirmAction)

	// Add swarm-specific tools
	ag.AddTool(tools.NewAskAgentTool(client))
	ag.AddTool(tools.NewBroadcastTool(client))
	ag.AddTool(tools.NewListAgentsTool(client))

	// Get role definition and inject swarm context
	roleDef := client.RoleDefinition()
	if roleDef != nil {
		// Prepend role-specific prompt
		ag.SetSystemPromptPrefix(fmt.Sprintf(`You are operating in HiveMind Swarm Mode as the %s (%s).

%s

Room: %s
Online agents will appear in the swarm panel.

To communicate with other agents, you can either:
1. Use @ROLE_NAME mentions in your response (e.g., @SA, @BE_DEV, @ALL)
2. Use the ask_agent tool to programmatically send messages

Available agent roles: ORCH, SA, BE_DEV, FE_DEV, QA, DEVOPS, DBA, SEC, HUMAN

`, roleDef.Name, role, roleDef.SystemPrompt, client.Room().Code))
	}

	// Create model name with role suffix
	modelDisplay := fmt.Sprintf("%s [%s]", modelName, role)

	// Start Swarm TUI with split-pane layout
	p := tea.NewProgram(
		tui.NewSwarmModel(ag, client, modelDisplay),
		tea.WithAltScreen(),
		tea.WithoutBracketedPaste(),
	)

	// Clean up on exit
	defer client.Close()

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	// Add swarm command to root
	rootCmd.AddCommand(swarmCmd)

	// Add subcommands to swarm
	swarmCmd.AddCommand(swarmCreateCmd)
	swarmCmd.AddCommand(swarmJoinCmd)
	swarmCmd.AddCommand(swarmListCmd)

	// Flags for create
	swarmCreateCmd.Flags().StringVar(&swarmNameFlag, "name", "", "Room name")
	swarmCreateCmd.Flags().StringVar(&swarmRepoFlag, "repo", "", "Project repository URL")

	// Flags for join
	swarmJoinCmd.Flags().StringVarP(&swarmRoleFlag, "role", "r", "", "Role to assume (required)")
	swarmJoinCmd.MarkFlagRequired("role")

	// Allow provider/model flags for swarm commands too
	swarmCmd.PersistentFlags().StringVarP(&providerFlag, "provider", "p", "", "LLM provider")
	swarmCmd.PersistentFlags().StringVarP(&modelFlag, "model", "m", "", "Model to use")
}
