package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/simonyos/Z-CODE/internal/swarm"
)

// AskAgentTool allows agents to send messages to other agents in the swarm
type AskAgentTool struct {
	BaseTool
	client *swarm.Client
}

// NewAskAgentTool creates a new ask_agent tool with swarm client
func NewAskAgentTool(client *swarm.Client) *AskAgentTool {
	return &AskAgentTool{
		BaseTool: BaseTool{
			Def: ToolDefinition{
				Name:        "ask_agent",
				Description: "Send a message to another agent in the swarm. Use this to collaborate with other specialized agents (ORCH, SA, BE_DEV, FE_DEV, QA, DEVOPS, DBA, SEC, HUMAN, ALL).",
				Parameters: &JSONSchema{
					Type: "object",
					Properties: map[string]*JSONSchema{
						"role": {
							Type:        "string",
							Description: "The target agent role (e.g., SA, BE_DEV, ORCH, ALL for broadcast)",
						},
						"message": {
							Type:        "string",
							Description: "The message to send to the agent",
						},
					},
					Required: []string{"role", "message"},
				},
			},
		},
		client: client,
	}
}

// Execute sends a message to the specified agent
func (t *AskAgentTool) Execute(ctx context.Context, args map[string]any) ToolResult {
	roleStr, _ := args["role"].(string)
	message, _ := args["message"].(string)

	if roleStr == "" {
		return ToolResult{Success: false, Error: "role is required"}
	}
	if message == "" {
		return ToolResult{Success: false, Error: "message is required"}
	}

	// Parse the role
	role, err := swarm.ParseRole(strings.ToUpper(roleStr))
	if err != nil {
		return ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid role '%s': valid roles are ORCH, SA, BE_DEV, FE_DEV, QA, DEVOPS, DBA, SEC, HUMAN, ALL", roleStr),
		}
	}

	// Send the message
	if role == swarm.RoleAll {
		err = t.client.Broadcast(message)
	} else {
		err = t.client.SendTo(role, message)
	}

	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("failed to send message: %v", err)}
	}

	return ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Message sent to %s", role),
	}
}

// BroadcastTool allows agents to broadcast messages to all agents
type BroadcastTool struct {
	BaseTool
	client *swarm.Client
}

// NewBroadcastTool creates a new broadcast tool
func NewBroadcastTool(client *swarm.Client) *BroadcastTool {
	return &BroadcastTool{
		BaseTool: BaseTool{
			Def: ToolDefinition{
				Name:        "broadcast",
				Description: "Send a message to all agents in the swarm",
				Parameters: &JSONSchema{
					Type: "object",
					Properties: map[string]*JSONSchema{
						"message": {
							Type:        "string",
							Description: "The message to broadcast to all agents",
						},
					},
					Required: []string{"message"},
				},
			},
		},
		client: client,
	}
}

// Execute broadcasts a message to all agents
func (t *BroadcastTool) Execute(ctx context.Context, args map[string]any) ToolResult {
	message, _ := args["message"].(string)

	if message == "" {
		return ToolResult{Success: false, Error: "message is required"}
	}

	if err := t.client.Broadcast(message); err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("failed to broadcast: %v", err)}
	}

	return ToolResult{
		Success: true,
		Output:  "Message broadcast to all agents",
	}
}

// ListAgentsTool allows agents to see who is online in the swarm
type ListAgentsTool struct {
	BaseTool
	client *swarm.Client
}

// NewListAgentsTool creates a new list_agents tool
func NewListAgentsTool(client *swarm.Client) *ListAgentsTool {
	return &ListAgentsTool{
		BaseTool: BaseTool{
			Def: ToolDefinition{
				Name:        "list_agents",
				Description: "List all agents currently online in the swarm",
				Parameters: &JSONSchema{
					Type:       "object",
					Properties: map[string]*JSONSchema{},
					Required:   []string{},
				},
			},
		},
		client: client,
	}
}

// Execute lists all online agents
func (t *ListAgentsTool) Execute(ctx context.Context, args map[string]any) ToolResult {
	roles := t.client.GetOnlineRoles()

	if len(roles) == 0 {
		return ToolResult{
			Success: true,
			Output:  "No other agents are currently online",
		}
	}

	var sb strings.Builder
	sb.WriteString("Online agents:\n")
	for _, role := range roles {
		if role == t.client.Role() {
			sb.WriteString(fmt.Sprintf("- %s (you)\n", role))
		} else {
			sb.WriteString(fmt.Sprintf("- %s\n", role))
		}
	}

	return ToolResult{
		Success: true,
		Output:  sb.String(),
	}
}
