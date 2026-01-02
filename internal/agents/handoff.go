package agents

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
)

// Pre-compiled regex for handoff parsing
var handoffPattern = regexp.MustCompile(`(?s)<handoff\s+agent="([^"]+)"(?:\s+reason="([^"]*)")?\s*>(.*?)</handoff>`)
var contextPattern = regexp.MustCompile(`(?s)<context\s+key="([^"]+)">(.*?)</context>`)

// ParseHandoff extracts a handoff instruction from agent response
func ParseHandoff(response string) *HandoffInstruction {
	matches := handoffPattern.FindStringSubmatch(response)
	if len(matches) < 4 {
		return nil
	}

	handoff := &HandoffInstruction{
		TargetAgent: matches[1],
		Reason:      matches[2],
		Context:     make(map[string]any),
	}

	// Parse context elements
	innerContent := matches[3]
	contextMatches := contextPattern.FindAllStringSubmatch(innerContent, -1)
	for _, cm := range contextMatches {
		if len(cm) >= 3 {
			key := cm[1]
			value := strings.TrimSpace(cm[2])
			handoff.Context[key] = value
		}
	}

	return handoff
}

// xmlHandoff is the XML structure for handoff instructions
type xmlHandoff struct {
	XMLName  xml.Name     `xml:"handoff"`
	Agent    string       `xml:"agent,attr"`
	Reason   string       `xml:"reason,attr"`
	Contexts []xmlContext `xml:"context"`
}

type xmlContext struct {
	Key   string `xml:"key,attr"`
	Value string `xml:",chardata"`
}

// FormatHandoff creates an XML handoff instruction string
func FormatHandoff(h *HandoffInstruction) string {
	var sb strings.Builder
	sb.WriteString("<handoff agent=\"")
	sb.WriteString(escapeAttr(h.TargetAgent))
	sb.WriteString("\"")
	if h.Reason != "" {
		sb.WriteString(" reason=\"")
		sb.WriteString(escapeAttr(h.Reason))
		sb.WriteString("\"")
	}
	sb.WriteString(">\n")

	for key, value := range h.Context {
		sb.WriteString("  <context key=\"")
		sb.WriteString(escapeAttr(key))
		sb.WriteString("\">")
		sb.WriteString(escapeXML(ValueToString(value)))
		sb.WriteString("</context>\n")
	}

	sb.WriteString("</handoff>")
	return sb.String()
}

// escapeAttr escapes special characters for XML attributes
func escapeAttr(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// escapeXML escapes special characters for XML content
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// ValueToString converts any value to string
// Exported so it can be used by workflows package
func ValueToString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}
