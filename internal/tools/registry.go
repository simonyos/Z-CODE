package tools

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Pre-compiled regex patterns for XML parsing (compiled once at package init)
var (
	multipleToolCallPattern = regexp.MustCompile(`(?s)<tool_calls>(.*?)</tool_calls>`)
	singleToolCallPattern   = regexp.MustCompile(`(?s)<tool_call>(.*?)</tool_call>`)
	openTagPattern          = regexp.MustCompile(`<(\w+)>`)
)

// Registry manages tool registration and execution
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry
func (r *Registry) Register(tool Tool) {
	def := tool.Definition()
	r.tools[def.Name] = tool
}

// Get retrieves a tool by name
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List returns all registered tool definitions
func (r *Registry) List() []ToolDefinition {
	defs := make([]ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Definition())
	}
	return defs
}

// Execute runs a tool by name with arguments
func (r *Registry) Execute(ctx context.Context, call ToolCall) ToolResult {
	tool, ok := r.Get(call.Name)
	if !ok {
		return ToolResult{Success: false, Error: fmt.Sprintf("unknown tool: %s", call.Name)}
	}

	if err := tool.Validate(call.Arguments); err != nil {
		return ToolResult{Success: false, Error: err.Error()}
	}

	return tool.Execute(ctx, call.Arguments)
}

// BuildSystemPrompt generates the tools section of system prompt
func (r *Registry) BuildSystemPrompt() string {
	cwd, _ := os.Getwd()

	var sb strings.Builder

	// Core identity and capabilities
	sb.WriteString("You are an expert software engineer and coding assistant.\n\n")
	sb.WriteString(fmt.Sprintf("Current working directory: %s\n\n", cwd))

	// Coding best practices
	sb.WriteString("CODING GUIDELINES:\n")
	sb.WriteString("- Write clean, maintainable code following best practices\n")
	sb.WriteString("- Prefer editing existing files over creating new ones\n")
	sb.WriteString("- Use edit_file for surgical changes instead of rewriting entire files\n")
	sb.WriteString("- Read files before modifying them to understand context\n")
	sb.WriteString("- Use glob and grep to explore the codebase before making changes\n")
	sb.WriteString("- Keep changes minimal and focused on the task\n")
	sb.WriteString("- Don't add unnecessary comments, logging, or error handling\n")
	sb.WriteString("- Follow the existing code style and patterns in the project\n")
	sb.WriteString("- Be careful with destructive operations (file writes, shell commands)\n\n")

	// Available tools
	sb.WriteString("AVAILABLE TOOLS:\n")

	for i, def := range r.List() {
		sb.WriteString(fmt.Sprintf("%d. %s - %s\n", i+1, def.Name, def.Description))

		// Generate example usage in XML format
		example := r.generateXMLExample(def, fmt.Sprintf("call_%d", i+1))
		sb.WriteString(fmt.Sprintf("   Example:\n%s\n\n", example))
	}

	sb.WriteString("TOOL FORMAT:\n")
	sb.WriteString("When using a tool, you MUST respond with properly formatted XML. Follow this EXACT structure:\n\n")
	sb.WriteString("<tool_call>\n")
	sb.WriteString("  <id>call_1</id>\n")
	sb.WriteString("  <name>tool_name</name>\n")
	sb.WriteString("  <parameters>\n")
	sb.WriteString("    <param_name>value</param_name>\n")
	sb.WriteString("  </parameters>\n")
	sb.WriteString("</tool_call>\n\n")

	sb.WriteString("For multiple tools at once (parallel execution), use <tool_calls> wrapper:\n\n")
	sb.WriteString("<tool_calls>\n")
	sb.WriteString("  <tool_call>\n")
	sb.WriteString("    <id>call_1</id>\n")
	sb.WriteString("    <name>read_file</name>\n")
	sb.WriteString("    <parameters>\n")
	sb.WriteString("      <path>README.md</path>\n")
	sb.WriteString("    </parameters>\n")
	sb.WriteString("  </tool_call>\n")
	sb.WriteString("  <tool_call>\n")
	sb.WriteString("    <id>call_2</id>\n")
	sb.WriteString("    <name>read_file</name>\n")
	sb.WriteString("    <parameters>\n")
	sb.WriteString("      <path>go.mod</path>\n")
	sb.WriteString("    </parameters>\n")
	sb.WriteString("  </tool_call>\n")
	sb.WriteString("</tool_calls>\n\n")

	sb.WriteString("CRITICAL: Each <tool_call> MUST contain <id>, <name>, and <parameters> elements. Never abbreviate or flatten the XML structure.\n\n")

	sb.WriteString("WORKFLOW:\n")
	sb.WriteString("1. If you need information, use tools first (read_file, glob, grep)\n")
	sb.WriteString("2. Each tool call must have a unique id (e.g., call_1, call_2)\n")
	sb.WriteString("3. You can call multiple tools in parallel when they're independent\n")
	sb.WriteString("4. After getting tool results, continue working or provide your answer\n")
	sb.WriteString("5. Be concise and helpful in your responses\n")
	sb.WriteString("6. When using tools, output ONLY the XML - no other text before or after\n")

	return sb.String()
}

func (r *Registry) generateXMLExample(def ToolDefinition, id string) string {
	var sb strings.Builder
	sb.WriteString("   <tool_call>\n")
	sb.WriteString(fmt.Sprintf("     <id>%s</id>\n", id))
	sb.WriteString(fmt.Sprintf("     <name>%s</name>\n", def.Name))
	sb.WriteString("     <parameters>\n")

	if def.Parameters != nil && def.Parameters.Properties != nil {
		// Sort parameter names for deterministic output
		names := make([]string, 0, len(def.Parameters.Properties))
		for name := range def.Parameters.Properties {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			sb.WriteString(fmt.Sprintf("       <%s>value</%s>\n", name, name))
		}
	}

	sb.WriteString("     </parameters>\n")
	sb.WriteString("   </tool_call>")
	return sb.String()
}

// xmlToolCall is the XML structure for parsing tool calls
type xmlToolCall struct {
	ID         string       `xml:"id"`
	Name       string       `xml:"name"`
	Parameters xmlParameters `xml:"parameters"`
}

// xmlParameters holds the dynamic parameter elements
type xmlParameters struct {
	Inner string `xml:",innerxml"`
}

// xmlToolCalls is the wrapper for multiple tool calls
type xmlToolCalls struct {
	Calls []xmlToolCall `xml:"tool_call"`
}

// ParseToolCalls extracts tool calls from XML in LLM response
// Returns a slice to support parallel tool calls
func ParseToolCalls(text string) ([]ToolCall, error) {
	text = strings.TrimSpace(text)

	var calls []ToolCall

	// Try to find <tool_calls> (multiple) first using pre-compiled pattern
	if matches := multipleToolCallPattern.FindStringSubmatch(text); len(matches) > 1 {
		xmlContent := "<tool_calls>" + matches[1] + "</tool_calls>"
		var wrapper xmlToolCalls
		if err := xml.Unmarshal([]byte(xmlContent), &wrapper); err != nil {
			return nil, fmt.Errorf("failed to parse tool_calls XML: %w", err)
		}
		for _, tc := range wrapper.Calls {
			call, err := parseXMLToolCall(tc)
			if err != nil {
				return nil, err
			}
			calls = append(calls, call)
		}
		return calls, nil
	}

	// Try to find single <tool_call> using pre-compiled pattern
	if matches := singleToolCallPattern.FindStringSubmatch(text); len(matches) > 1 {
		xmlContent := "<tool_call>" + matches[1] + "</tool_call>"
		var tc xmlToolCall
		if err := xml.Unmarshal([]byte(xmlContent), &tc); err != nil {
			return nil, fmt.Errorf("failed to parse tool_call XML: %w", err)
		}
		call, err := parseXMLToolCall(tc)
		if err != nil {
			return nil, err
		}
		return []ToolCall{call}, nil
	}

	return nil, fmt.Errorf("no tool_call or tool_calls XML found")
}

// parseXMLToolCall converts an xmlToolCall to a ToolCall
func parseXMLToolCall(tc xmlToolCall) (ToolCall, error) {
	if tc.Name == "" {
		return ToolCall{}, fmt.Errorf("missing tool name")
	}
	if tc.ID == "" {
		return ToolCall{}, fmt.Errorf("missing tool id")
	}

	// Parse parameters from inner XML
	// Note: This simple parser handles flat parameter elements. It does not support
	// nested XML elements, CDATA sections, or XML entities within parameter values.
	// Parameter values containing closing tags (e.g., "</path>") will not parse correctly.
	args := make(map[string]any)
	if tc.Parameters.Inner != "" {
		// Parse each parameter element using pre-compiled pattern
		// Find opening tags, then find corresponding closing tags
		inner := tc.Parameters.Inner

		openMatches := openTagPattern.FindAllStringSubmatchIndex(inner, -1)

		for _, match := range openMatches {
			if len(match) >= 4 {
				tagEnd := match[1]
				nameStart := match[2]
				nameEnd := match[3]

				paramName := inner[nameStart:nameEnd]
				closeTag := "</" + paramName + ">"

				// Find the closing tag
				closeIdx := strings.Index(inner[tagEnd:], closeTag)
				if closeIdx != -1 {
					paramValue := strings.TrimSpace(inner[tagEnd : tagEnd+closeIdx])
					args[paramName] = paramValue
				}
			}
		}
	}

	return ToolCall{
		ID:        tc.ID,
		Name:      tc.Name,
		Arguments: args,
	}, nil
}

// ParseToolCall is kept for backward compatibility, returns first tool call.
//
// Deprecated: Use ParseToolCalls instead for proper support of multiple tool calls.
func ParseToolCall(text string) (*ToolCall, error) {
	calls, err := ParseToolCalls(text)
	if err != nil {
		return nil, err
	}
	if len(calls) == 0 {
		return nil, fmt.Errorf("no tool calls found")
	}
	return &calls[0], nil
}

// escapeXML escapes special XML characters in a string using the standard library
func escapeXML(s string) string {
	var buf strings.Builder
	if err := xml.EscapeText(&buf, []byte(s)); err != nil {
		// Fallback to manual escaping on error (shouldn't happen with string input)
		s = strings.ReplaceAll(s, "&", "&amp;")
		s = strings.ReplaceAll(s, "<", "&lt;")
		s = strings.ReplaceAll(s, ">", "&gt;")
		s = strings.ReplaceAll(s, "\"", "&quot;")
		s = strings.ReplaceAll(s, "'", "&apos;")
		return s
	}
	return buf.String()
}

// FormatToolResult formats a tool result as XML to send back to the LLM
func FormatToolResult(id, name string, result ToolResult) string {
	if result.Success {
		return fmt.Sprintf(`<tool_result id="%s" name="%s" success="true">
  <output>%s</output>
</tool_result>`, escapeXML(id), escapeXML(name), escapeXML(result.Output))
	}
	return fmt.Sprintf(`<tool_result id="%s" name="%s" success="false">
  <error>%s</error>
</tool_result>`, escapeXML(id), escapeXML(name), escapeXML(result.Error))
}

// FormatToolResults formats multiple tool results as XML
func FormatToolResults(results []struct {
	ID     string
	Name   string
	Result ToolResult
}) string {
	if len(results) == 1 {
		return FormatToolResult(results[0].ID, results[0].Name, results[0].Result)
	}

	var sb strings.Builder
	sb.WriteString("<tool_results>\n")
	for _, r := range results {
		sb.WriteString(FormatToolResult(r.ID, r.Name, r.Result))
		sb.WriteString("\n")
	}
	sb.WriteString("</tool_results>")
	return sb.String()
}
