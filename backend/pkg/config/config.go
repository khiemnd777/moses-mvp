package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type VectorRepairConfig struct {
	Enabled         bool          `yaml:"enabled"`
	Interval        time.Duration `yaml:"interval"`
	MaxTasksPerPass int           `yaml:"max_tasks_per_pass"`
}

type Config struct {
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
	Postgres struct {
		DSN string `yaml:"dsn"`
	} `yaml:"postgres"`
	Qdrant struct {
		URL        string `yaml:"url"`
		Collection string `yaml:"collection"`
	} `yaml:"qdrant"`
	OpenAI struct {
		APIKey          string `yaml:"api_key"`
		EmbeddingsModel string `yaml:"embeddings_model"`
		ChatModel       string `yaml:"chat_model"`
	} `yaml:"openai"`
	Storage struct {
		RootDir string `yaml:"root_dir"`
	} `yaml:"storage"`
	Ingest struct {
		DefaultSegmenter string `yaml:"default_segmenter"`
		ChunkSize        int    `yaml:"chunk_size"`
		ChunkOverlap     int    `yaml:"chunk_overlap"`
	} `yaml:"ingest"`
	Prompts struct {
		Guard         string `yaml:"guard"`
		ToneDefault   string `yaml:"tone_default"`
		ToneAcademic  string `yaml:"tone_academic"`
		ToneProcedure string `yaml:"tone_procedure"`
	} `yaml:"prompts"`
	VectorRepair VectorRepairConfig `yaml:"vector_repair"`
}

func Load(path string) (Config, error) {
	cfg := Config{
		VectorRepair: VectorRepairConfig{
			Enabled:         true,
			Interval:        30 * time.Second,
			MaxTasksPerPass: 20,
		},
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
