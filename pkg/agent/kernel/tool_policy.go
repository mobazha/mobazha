package kernel

import (
	"context"
	"errors"
	"time"
)

// ToolGrant describes the active skill and acting persona requesting tool
// access for a turn.
type ToolGrant struct {
	SkillID      SkillID
	Capabilities []Capability
	Persona      Persona
}

// StaticToolCatalog is a simple in-process catalog implementation suitable for
// open-source kernels and tests. Private deployments can replace it with a
// dynamic policy provider without changing the runtime.
type StaticToolCatalog struct {
	tools []ToolMetadata
}

// NewStaticToolCatalog creates a catalog from immutable tool metadata.
func NewStaticToolCatalog(tools []ToolMetadata) *StaticToolCatalog {
	cp := make([]ToolMetadata, len(tools))
	copy(cp, tools)
	return &StaticToolCatalog{tools: cp}
}

func (c *StaticToolCatalog) List(_ context.Context, scope Scope) ([]ToolMetadata, error) {
	if c == nil {
		return nil, nil
	}
	out := make([]ToolMetadata, 0, len(c.tools))
	for _, tool := range c.tools {
		if scope.ActingPersona != "" && !personaAllowed(tool.AllowedPersonas, scope.ActingPersona) {
			continue
		}
		out = append(out, cloneToolMetadata(tool))
	}
	return out, nil
}

func (c *StaticToolCatalog) Resolve(ctx context.Context, scope Scope, name string) (*ToolMetadata, error) {
	tools, err := c.List(ctx, scope)
	if err != nil {
		return nil, err
	}
	for _, tool := range tools {
		if tool.Name == name {
			cp := cloneToolMetadata(tool)
			return &cp, nil
		}
	}
	return nil, errors.New("agent: tool not found")
}

// FilterToolsForGrant returns catalog tools usable by the active skill under
// the current persona. It intentionally treats tool hints as non-authoritative;
// hints can influence prompt wording, but this function is the policy gate.
func FilterToolsForGrant(tools []ToolMetadata, grant ToolGrant) []ToolMetadata {
	out := make([]ToolMetadata, 0, len(tools))
	for _, tool := range tools {
		if !ToolAllowedForGrant(tool, grant) {
			continue
		}
		out = append(out, cloneToolMetadata(tool))
	}
	return out
}

// ToolAllowedForGrant evaluates a single catalog entry against a skill grant.
func ToolAllowedForGrant(tool ToolMetadata, grant ToolGrant) bool {
	if grant.Persona != "" && !personaAllowed(tool.AllowedPersonas, grant.Persona) {
		return false
	}
	if len(tool.AllowedSkills) > 0 && !skillAllowed(tool.AllowedSkills, grant.SkillID) {
		return false
	}
	if len(tool.Capabilities) > 0 {
		if len(grant.Capabilities) == 0 || !capabilityIntersects(tool.Capabilities, grant.Capabilities) {
			return false
		}
	}
	if len(tool.AllowedSkills) == 0 && len(tool.Capabilities) == 0 {
		return grant.SkillID == "" && len(grant.Capabilities) == 0
	}
	return true
}

func skillAllowed(allowed []SkillID, id SkillID) bool {
	if id == "" {
		return false
	}
	for _, item := range allowed {
		if item == id {
			return true
		}
	}
	return false
}

func personaAllowed(allowed []Persona, persona Persona) bool {
	if len(allowed) == 0 || persona == "" {
		return true
	}
	for _, item := range allowed {
		if item == persona {
			return true
		}
	}
	return false
}

func capabilityIntersects(toolCaps []Capability, requested []Capability) bool {
	if len(requested) == 0 {
		return true
	}
	if len(toolCaps) == 0 {
		return false
	}
	seen := make(map[Capability]struct{}, len(toolCaps))
	for _, cap := range toolCaps {
		seen[cap] = struct{}{}
	}
	for _, cap := range requested {
		if _, ok := seen[cap]; ok {
			return true
		}
	}
	return false
}

func cloneToolMetadata(tool ToolMetadata) ToolMetadata {
	cp := tool
	cp.InputSchema = cloneRawMessage(tool.InputSchema)
	cp.OutputSchema = cloneRawMessage(tool.OutputSchema)
	cp.Capabilities = append([]Capability(nil), tool.Capabilities...)
	cp.AllowedSkills = append([]SkillID(nil), tool.AllowedSkills...)
	cp.AllowedPersonas = append([]Persona(nil), tool.AllowedPersonas...)
	if tool.Timeout < 0 {
		cp.Timeout = 0 * time.Second
	}
	return cp
}

func cloneRawMessage(in []byte) []byte {
	if len(in) == 0 {
		return nil
	}
	out := make([]byte, len(in))
	copy(out, in)
	return out
}
