package concourse

type Source struct {
	Target   string `json:"target"`
	Teams    []Team `json:"teams"`
	Insecure string `json:"insecure"`
}

type Team struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type CheckRequest struct {
	Source  Source  `json:"source"`
	Version Version `json:"version"`
}

type Version map[string]string

type CheckResponse []Version

type InRequest struct {
	Source  Source   `json:"source"`
	Version Version  `json:"version"`
	Params  InParams `json:"params"`
}

type InParams struct {
}

type InResponse struct {
	Version  Version    `json:"version"`
	Metadata []Metadata `json:"metadata"`
}

type Metadata struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type OutRequest struct {
	Source Source    `json:"source"`
	Params OutParams `json:"params"`
}

type OutParams struct {
	Pipelines     []Pipeline `json:"pipelines,omitempty"`
	PipelinesFile string     `json:"pipelines_file,omitempty"`
}

type Pipeline struct {
	Name       string                 `json:"name" yaml:"name"`
	ConfigFile string                 `json:"config_file" yaml:"config_file"`
	VarsFiles  []string               `json:"vars_files" yaml:"vars_files"`
	Vars       map[string]interface{} `json:"vars" yaml:"vars"`
	TeamName   string                 `json:"team" yaml:"team"`
	Unpaused   bool                   `json:"unpaused" yaml:"unpaused"`
	Exposed    bool                   `json:"exposed" yaml:"exposed"`
	HideStdout bool                   `json:"hide_stdout" yaml:"hide_stdout"`
}

type OutResponse struct {
	Version  Version    `json:"version"`
	Metadata []Metadata `json:"metadata"`
}
