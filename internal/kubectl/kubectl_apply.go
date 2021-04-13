package kubectl

type resource struct {
	Kind     string   `yaml:"kind"`
	Metadata metadata `yaml:"metadata"`

	Items []resource `yaml:"items"`
}

type metadata struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}
