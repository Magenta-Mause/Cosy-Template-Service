package models

import (
	"math"
	"strings"
)

// containsVar reports whether a string carries an unresolved {{var}} template
// placeholder. v1/v2 use typed fields that cannot hold placeholders, so any
// value containing "{{" is omitted from the v1/v2 output.
func containsVar(s string) bool {
	return strings.Contains(s, "{{")
}

// GamesIndex maps a game slug (games/*.yaml filename without extension) to its
// definition. It is used to resolve a slug game_id to a numeric external id for
// v1/v2 output.
type GamesIndex map[string]Game

// ResourceLimitV1 is the typed (old) resource limit shape used by v1/v2. CPU is
// a plain number and Memory a plain string; any value carrying a {{var}}
// placeholder is omitted entirely.
type ResourceLimitV1 struct {
	Memory string  `json:"memory,omitempty"`
	CPU    float64 `json:"cpu,omitempty"`
}

type VariableV1 struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Regex       string   `json:"regex,omitempty"`
	Placeholder string   `json:"placeholder"`
	Default     any      `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Example     any      `json:"example,omitempty"`
	Description string   `json:"description,omitempty"`
}

type VariableV2 struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Regex        string   `json:"regex,omitempty"`
	Placeholder  string   `json:"placeholder"`
	DefaultValue any      `json:"default_value,omitempty"`
	Options      []string `json:"options,omitempty"`
	Required     bool     `json:"required,omitempty"`
	Example      any      `json:"example,omitempty"`
	Description  string   `json:"description,omitempty"`
}

type TemplateV1 struct {
	Name                   string            `json:"name"`
	Description            string            `json:"description"`
	Path                   string            `json:"path,omitempty"`
	Variables              []VariableV1      `json:"variables,omitempty"`
	GameID                 *int              `json:"game_id,omitempty"`
	DockerImageName        string            `json:"docker_image_name"`
	DockerImageTag         string            `json:"docker_image_tag"`
	DockerExecutionCommand []string          `json:"docker_execution_command,omitempty"`
	EnvironmentVariables   map[string]string `json:"environment_variables,omitempty"`
	PortMapping            map[string]any    `json:"port_mapping,omitempty"`
	FileMounts             []string          `json:"file_mounts,omitempty"`
	ResourceLimit          *ResourceLimitV1  `json:"resource_limit,omitempty"`
	Tags                   []string          `json:"tags,omitempty"`
}

type TemplateV2 struct {
	Name                   string            `json:"name"`
	Description            string            `json:"description"`
	Path                   string            `json:"path,omitempty"`
	Variables              []VariableV2      `json:"variables,omitempty"`
	GameID                 *int              `json:"game_id,omitempty"`
	DockerImageName        string            `json:"docker_image_name"`
	DockerImageTag         string            `json:"docker_image_tag"`
	DockerExecutionCommand []string          `json:"docker_execution_command,omitempty"`
	EnvironmentVariables   map[string]string `json:"environment_variables,omitempty"`
	PortMapping            map[string]any    `json:"port_mapping,omitempty"`
	FileMounts             []string          `json:"file_mounts,omitempty"`
	ResourceLimit          *ResourceLimitV1  `json:"resource_limit,omitempty"`
	Tags                   []string          `json:"tags,omitempty"`
}

// resolveGameID resolves a template's game_id to a numeric external id for
// v1/v2 output. If the game_id is already numeric it is passed through. If it is
// a slug it is resolved via the games index to games[slug].ExternalGameID. If
// the slug is unknown or has no external_game_id, nil is returned so the field
// is omitted gracefully rather than emitting a broken value.
func resolveGameID(id StringOrNumber, games GamesIndex) *int {
	switch v := id.Value().(type) {
	case nil:
		return nil
	case int64:
		if v < 0 {
			return nil
		}
		n := int(v)
		return &n
	case float64:
		if v != math.Trunc(v) || v < 0 {
			return nil
		}
		n := int(v)
		return &n
	case string:
		g, ok := games[v]
		if !ok || g.ExternalGameID == nil {
			return nil
		}
		n := *g.ExternalGameID
		return &n
	default:
		return nil
	}
}

// resourceLimitV1 converts the v3 resource limit to the typed v1/v2 shape,
// omitting any field whose value still contains a {{var}} placeholder. Returns
// nil if the result would be empty.
func resourceLimitV1(rl *ResourceLimit) *ResourceLimitV1 {
	if rl == nil {
		return nil
	}
	out := ResourceLimitV1{}
	empty := true

	if mem := rl.Memory.String(); mem != "" && !containsVar(mem) {
		out.Memory = mem
		empty = false
	}
	if !rl.CPU.IsZero() && !containsVar(rl.CPU.String()) {
		switch v := rl.CPU.Value().(type) {
		case int64:
			out.CPU = float64(v)
			empty = false
		case float64:
			out.CPU = v
			empty = false
		}
	}
	if empty {
		return nil
	}
	return &out
}

// portMappingV1 converts the v3 port mapping to the typed v1/v2 shape, omitting
// any entry whose value contains a {{var}} placeholder. Numeric values are
// emitted as JSON numbers (int64/float64). Returns nil if the result is empty.
func portMappingV1(pm map[string]StringOrNumber) map[string]any {
	if pm == nil {
		return nil
	}
	out := make(map[string]any, len(pm))
	for k, v := range pm {
		switch val := v.Value().(type) {
		case string:
			if containsVar(val) {
				continue
			}
			out[k] = val
		default:
			out[k] = val
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ToV1 converts the raw template to the v1 wire shape ("default" variable key),
// resolving game_id via the games index and dropping new-only fields.
func (t *Template) ToV1(games GamesIndex) TemplateV1 {
	vars := make([]VariableV1, len(t.Variables))
	for i, v := range t.Variables {
		vars[i] = VariableV1{
			Name:        v.Name,
			Type:        v.Type,
			Regex:       v.Regex,
			Placeholder: v.Placeholder,
			Default:     v.Default,
			Options:     v.Options,
			Required:    v.Required,
			Example:     v.Example,
			Description: v.Description,
		}
	}
	return TemplateV1{
		Name:                   t.Name,
		Description:            t.Description,
		Path:                   t.Path,
		Variables:              vars,
		GameID:                 resolveGameID(t.GameID, games),
		DockerImageName:        t.DockerImageName,
		DockerImageTag:         t.DockerImageTag,
		DockerExecutionCommand: t.DockerExecutionCommand,
		EnvironmentVariables:   t.EnvironmentVariables,
		PortMapping:            portMappingV1(t.PortMapping),
		FileMounts:             t.FileMounts,
		ResourceLimit:          resourceLimitV1(t.ResourceLimit),
		Tags:                   t.Tags,
	}
}

// ToV2 converts the raw template to the v2 wire shape ("default_value" variable
// key), resolving game_id via the games index and dropping new-only fields.
func (t *Template) ToV2(games GamesIndex) TemplateV2 {
	vars := make([]VariableV2, len(t.Variables))
	for i, v := range t.Variables {
		vars[i] = VariableV2{
			Name:         v.Name,
			Type:         v.Type,
			Regex:        v.Regex,
			Placeholder:  v.Placeholder,
			DefaultValue: v.Default,
			Options:      v.Options,
			Required:     v.Required,
			Example:      v.Example,
			Description:  v.Description,
		}
	}
	return TemplateV2{
		Name:                   t.Name,
		Description:            t.Description,
		Path:                   t.Path,
		Variables:              vars,
		GameID:                 resolveGameID(t.GameID, games),
		DockerImageName:        t.DockerImageName,
		DockerImageTag:         t.DockerImageTag,
		DockerExecutionCommand: t.DockerExecutionCommand,
		EnvironmentVariables:   t.EnvironmentVariables,
		PortMapping:            portMappingV1(t.PortMapping),
		FileMounts:             t.FileMounts,
		ResourceLimit:          resourceLimitV1(t.ResourceLimit),
		Tags:                   t.Tags,
	}
}
