package prompt

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Prompt struct {
	Name    string `yaml:"name"`
	Content string `yaml:"content"`
}

func Load(path string) (Prompt, error) {
	var p Prompt
	b, err := os.ReadFile(path)
	if err != nil {
		return p, err
	}
	if err := yaml.Unmarshal(b, &p); err != nil {
		return p, err
	}
	return p, nil
}
