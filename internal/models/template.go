package models

type Variable struct {
	Name        string   `yaml:"name" json:"name"`
	Type        string   `yaml:"type" json:"type"`
	Regex       string   `yaml:"regex,omitempty" json:"regex,omitempty"`
	Placeholder string   `yaml:"placeholder" json:"placeholder"`
	Default     any      `yaml:"default,omitempty" json:"default,omitempty"`
	Options     []string `yaml:"options,omitempty" json:"options,omitempty"`
	Required    bool     `yaml:"required,omitempty" json:"required,omitempty"`
}

type ResourceLimit struct {
	Memory string  `yaml:"memory,omitempty" json:"memory,omitempty"`
	CPU    float64 `yaml:"cpu,omitempty" json:"cpu,omitempty"`
}

type Template struct {
	Name                   string            `yaml:"name" json:"name"`
	Description            string            `yaml:"description" json:"description"`
	Variables              []Variable        `yaml:"variables,omitempty" json:"variables,omitempty"`
	GameID                 int               `yaml:"game_id" json:"game_id"`
	DockerImageName        string            `yaml:"docker_image_name" json:"docker_image_name"`
	DockerImageTag         string            `yaml:"docker_image_tag" json:"docker_image_tag"`
	DockerExecutionCommand *string           `yaml:"docker_execution_command,omitempty" json:"docker_execution_command,omitempty"`
	EnvironmentVariables   map[string]string `yaml:"environment_variables,omitempty" json:"environment_variables,omitempty"`
	PortMapping            map[string]any    `yaml:"port_mapping,omitempty" json:"port_mapping,omitempty"`
	FileMounts             []string          `yaml:"file_mounts,omitempty" json:"file_mounts,omitempty"`
	ResourceLimit          *ResourceLimit    `yaml:"resource_limit,omitempty" json:"resource_limit,omitempty"`
}
