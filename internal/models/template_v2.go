package models

type VariableV2 struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Regex        string   `json:"regex,omitempty"`
	Placeholder  string   `json:"placeholder"`
	DefaultValue any      `json:"default_value,omitempty"`
	Options      []string `json:"options,omitempty"`
	Required     bool     `json:"required,omitempty"`
	Example      any      `json:"example,omitempty"`
}

type TemplateV2 struct {
	Name                   string            `json:"name"`
	Description            string            `json:"description"`
	Path                   string            `json:"path,omitempty"`
	Variables              []VariableV2      `json:"variables,omitempty"`
	GameID                 int               `json:"game_id"`
	DockerImageName        string            `json:"docker_image_name"`
	DockerImageTag         string            `json:"docker_image_tag"`
	DockerExecutionCommand []string          `json:"docker_execution_command,omitempty"`
	EnvironmentVariables   map[string]string `json:"environment_variables,omitempty"`
	PortMapping            map[string]any    `json:"port_mapping,omitempty"`
	FileMounts             []string          `json:"file_mounts,omitempty"`
	ResourceLimit          *ResourceLimit    `json:"resource_limit,omitempty"`
}

func (t *Template) ToV2() TemplateV2 {
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
		}
	}
	return TemplateV2{
		Name:                   t.Name,
		Description:            t.Description,
		Path:                   t.Path,
		Variables:              vars,
		GameID:                 t.GameID,
		DockerImageName:        t.DockerImageName,
		DockerImageTag:         t.DockerImageTag,
		DockerExecutionCommand: t.DockerExecutionCommand,
		EnvironmentVariables:   t.EnvironmentVariables,
		PortMapping:            t.PortMapping,
		FileMounts:             t.FileMounts,
		ResourceLimit:          t.ResourceLimit,
	}
}
