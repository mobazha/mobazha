package skill

import (
	"fmt"
	"sort"
	"strings"
)

// PromptTool is a sanitized tool summary injected for an active skill.
type PromptTool struct {
	Name        string
	Description string
	Risk        string
	Approval    string
}

// PromptContextOptions carries the skill context that may be injected into the
// system prompt for a single turn.
type PromptContextOptions struct {
	Available    []string
	Active       []*Skill
	GrantedTools map[string][]PromptTool
}

// BuildPromptContext formats runtime-loaded skills for injection into the
// system prompt. Available skills are summarized; active skills include full
// Markdown bodies as the source of truth for the current turn.
func BuildPromptContext(available []string, active []*Skill) string {
	return BuildPromptContextWithOptions(PromptContextOptions{
		Available: available,
		Active:    active,
	})
}

// BuildPromptContextWithOptions formats runtime-loaded skills and authorized
// tools. Tool hints inside a skill are planning hints only; granted tools are
// computed by runtime policy and shown separately.
func BuildPromptContextWithOptions(opts PromptContextOptions) string {
	available := opts.Available
	active := opts.Active
	if len(available) == 0 && len(active) == 0 {
		return ""
	}
	var b strings.Builder
	if len(available) > 0 {
		b.WriteString("## Available Skills\n\n")
		b.WriteString("If the user request strongly matches an available skill, call `use_skill_tool` before ordinary tools and use the returned skill content as guidance. If no skill clearly matches, do not guess.\n\n")
		for i, id := range available {
			fmt.Fprintf(&b, "%d. `%s`\n", i+1, id)
		}
		b.WriteString("\n")
	}
	if len(active) > 0 {
		b.WriteString("## Runtime-Injected Active Skills\n\n")
		b.WriteString("The following skill definitions are loaded dynamically for this turn. Use them as the source of truth for skill-specific workflow, policy, and tool orchestration.\n\n")
		b.WriteString("<<<ACTIVE_SKILLS_START>>>\n")
		for _, s := range active {
			if s == nil {
				continue
			}
			fmt.Fprintf(&b, "### `%s`\n\n", s.Name)
			if s.Location != "" {
				fmt.Fprintf(&b, "location: `%s`\n\n", s.Location)
			}
			if persona := s.Persona(); persona != "" {
				fmt.Fprintf(&b, "persona: `%s`\n\n", persona)
			}
			if caps := s.Capabilities(); len(caps) > 0 {
				fmt.Fprintf(&b, "required capabilities: %s\n\n", formatCodeList(caps))
			}
			if hints := s.ToolHints(); len(hints) > 0 {
				fmt.Fprintf(&b, "tool hints: %s. These are non-authoritative; use only granted tools.\n\n", formatCodeList(hints))
			}
			if tools := opts.GrantedTools[s.ID]; len(tools) > 0 {
				b.WriteString("granted tools for this turn:\n")
				for _, tool := range tools {
					fmt.Fprintf(&b, "- `%s`", tool.Name)
					if tool.Risk != "" || tool.Approval != "" {
						fmt.Fprintf(&b, " (risk: %s, approval: %s)", tool.Risk, tool.Approval)
					}
					if tool.Description != "" {
						fmt.Fprintf(&b, ": %s", tool.Description)
					}
					b.WriteString("\n")
				}
				b.WriteString("\n")
			}
			if s.Description != "" {
				b.WriteString(s.Description)
				b.WriteString("\n\n")
			}
			if s.Content != "" {
				b.WriteString(s.Content)
				if !strings.HasSuffix(s.Content, "\n") {
					b.WriteString("\n")
				}
				b.WriteString("\n")
			}
		}
		b.WriteString("<<<ACTIVE_SKILLS_END>>>")
	}
	return strings.TrimSpace(b.String())
}

func formatCodeList(items []string) string {
	cp := append([]string(nil), items...)
	sort.Strings(cp)
	for i, item := range cp {
		cp[i] = "`" + item + "`"
	}
	return strings.Join(cp, ", ")
}
