package models

import "encoding/json"

type Variable struct {
	Name        string   `yaml:"name" json:"name"`
	Type        string   `yaml:"type" json:"type"`
	Regex       string   `yaml:"regex,omitempty" json:"regex,omitempty"`
	Placeholder string   `yaml:"placeholder" json:"placeholder"`
	Default     any      `yaml:"default,omitempty" json:"default,omitempty"`
	Options     []string `yaml:"options,omitempty" json:"options,omitempty"`
	Required    bool     `yaml:"required,omitempty" json:"required,omitempty"`
	Example     any      `yaml:"example,omitempty" json:"example,omitempty"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
}

// ResourceLimit holds container resource limits. CPU and Memory are
// string-or-number scalars so they can carry {{var}} placeholder strings in v3
// while still round-tripping plain numeric/string values.
type ResourceLimit struct {
	Memory StringOrNumber `yaml:"memory,omitempty" json:"memory,omitzero"`
	CPU    StringOrNumber `yaml:"cpu,omitempty" json:"cpu,omitzero"`
}

// HostMount is a direct host volume bind mount (admin/owner only at runtime).
// ReadOnly is a pointer so an omitted read_only can be distinguished from an
// explicit false; the schema default for an omitted value is true. The wire
// output always carries an explicit boolean (see MarshalJSON) so the downstream
// consumer never has to reapply the default.
type HostMount struct {
	HostPath      string `yaml:"host_path" json:"host_path"`
	ContainerPath string `yaml:"container_path" json:"container_path"`
	ReadOnly      *bool  `yaml:"read_only,omitempty" json:"read_only"`
}

// EffectiveReadOnly returns the read-only flag with the schema default (true)
// applied when read_only was omitted in the source YAML.
func (m HostMount) EffectiveReadOnly() bool {
	if m.ReadOnly == nil {
		return true
	}
	return *m.ReadOnly
}

func (m HostMount) MarshalJSON() ([]byte, error) {
	type alias struct {
		HostPath      string `json:"host_path"`
		ContainerPath string `json:"container_path"`
		ReadOnly      bool   `json:"read_only"`
	}
	return json.Marshal(alias{
		HostPath:      m.HostPath,
		ContainerPath: m.ContainerPath,
		ReadOnly:      m.EffectiveReadOnly(),
	})
}

type Template struct {
	Name                   string                    `yaml:"name" json:"name"`
	Description            string                    `yaml:"description" json:"description"`
	Path                   string                    `yaml:"path,omitempty" json:"path,omitempty"`
	Variables              []Variable                `yaml:"variables,omitempty" json:"variables,omitempty"`
	GameID                 StringOrNumber            `yaml:"game_id" json:"game_id"`
	DockerImageName        string                    `yaml:"docker_image_name" json:"docker_image_name"`
	DockerImageTag         string                    `yaml:"docker_image_tag" json:"docker_image_tag"`
	DockerExecutionCommand []string                  `yaml:"docker_execution_command,omitempty" json:"docker_execution_command,omitempty"`
	EnvironmentVariables   map[string]string         `yaml:"environment_variables,omitempty" json:"environment_variables,omitempty"`
	PortMapping            map[string]StringOrNumber `yaml:"port_mapping,omitempty" json:"port_mapping,omitempty"`
	FileMounts             []string                  `yaml:"file_mounts,omitempty" json:"file_mounts,omitempty"`
	HostMounts             []HostMount               `yaml:"host_mounts,omitempty" json:"host_mounts,omitempty"`
	Annotations            map[string]string         `yaml:"annotations,omitempty" json:"annotations,omitempty"`
	ResourceLimit          *ResourceLimit            `yaml:"resource_limit,omitempty" json:"resource_limit,omitempty"`
	Tags                   []string                  `yaml:"tags,omitempty" json:"tags,omitempty"`
}
