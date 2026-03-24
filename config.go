package main

import (
	"encoding/json"
	"os"
)

// Config holds all runtime settings. Values are loaded from config.json
// and can be overridden by environment variables in future iterations.
type Config struct {
	QdrantHost        string `json:"qdrant_host"`
	QdrantPort        int    `json:"qdrant_port"`
	OllamaURL         string `json:"ollama_url"`
	EmbedModel        string `json:"embed_model"`
	ChatModel         string `json:"chat_model"`
	VectorSize        int    `json:"vector_size"`
	APIPort           int    `json:"api_port"`
	DefaultTopK       int    `json:"default_top_k"`
	DefaultCollection string `json:"default_collection"`
}

func defaultConfig() Config {
	return Config{
		QdrantHost:        "localhost",
		QdrantPort:        6334,
		OllamaURL:         "http://localhost:11434",
		EmbedModel:        "nomic-embed-text",
		ChatModel:         "llama3.2",
		VectorSize:        768,
		APIPort:           8080,
		DefaultTopK:       5,
		DefaultCollection: "constitution",
	}
}

// LoadConfig reads config.json from the given path; falls back to defaults
// if the file is missing so the binary works out-of-the-box.
func LoadConfig(path string) (Config, error) {
	cfg := defaultConfig()

	f, err := os.Open(path)
	if err != nil {
		return cfg, nil // file missing → use defaults silently
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
