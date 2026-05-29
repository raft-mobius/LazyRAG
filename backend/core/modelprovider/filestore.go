package modelprovider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"lazymind/core/log"
)

// FileModelConfig is the top-level structure persisted in model_config.json.
type FileModelConfig struct {
	Providers      []FileProvider      `json:"providers"`
	SelectedModels []FileSelectedModel `json:"selected_models"`
}

type FileProvider struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	BaseURL     string      `json:"base_url"`
	Groups      []FileGroup `json:"groups"`
}

type FileGroup struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	BaseURL    string      `json:"base_url"`
	APIKey     string      `json:"api_key"`
	IsVerified bool        `json:"is_verified"`
	Models     []FileModel `json:"models"`
}

type FileModel struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ModelType string `json:"model_type"`
	BaseURL   string `json:"base_url"`
	IsDefault bool   `json:"is_default"`
}

type FileSelectedModel struct {
	ModelType string `json:"model_type"`
	ModelID   string `json:"model_id"`
}

// fileStore holds the in-memory state and synchronizes access.
var fileStore struct {
	mu   sync.RWMutex
	path string
	cfg  *FileModelConfig
}

// InitFileStore loads the model config file into memory. Call once at startup in desktop mode.
func InitFileStore(path string) {
	fileStore.mu.Lock()
	defer fileStore.mu.Unlock()

	fileStore.path = path
	cfg, err := loadModelConfigFile(path)
	if err != nil {
		log.Logger.Warn().Err(err).Str("path", path).Msg("load model_config.json failed; starting with empty config")
		cfg = &FileModelConfig{
			Providers:      []FileProvider{},
			SelectedModels: []FileSelectedModel{},
		}
	}
	fileStore.cfg = cfg
	log.Logger.Info().Str("path", path).Int("providers", len(cfg.Providers)).Int("selections", len(cfg.SelectedModels)).Msg("file model config loaded")
}

func getFileConfig() *FileModelConfig {
	fileStore.mu.RLock()
	defer fileStore.mu.RUnlock()
	return fileStore.cfg
}

func updateFileConfig(fn func(cfg *FileModelConfig)) error {
	fileStore.mu.Lock()
	defer fileStore.mu.Unlock()

	fn(fileStore.cfg)
	return saveModelConfigFile(fileStore.path, fileStore.cfg)
}

func loadModelConfigFile(path string) (*FileModelConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg FileModelConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse model_config.json: %w", err)
	}
	if cfg.Providers == nil {
		cfg.Providers = []FileProvider{}
	}
	if cfg.SelectedModels == nil {
		cfg.SelectedModels = []FileSelectedModel{}
	}
	return &cfg, nil
}

func saveModelConfigFile(path string, cfg *FileModelConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal model_config.json: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".model_config_*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// BuildLLMConfigFromFile resolves selected models into the map format expected by the algorithm service.
func BuildLLMConfigFromFile() map[string]any {
	cfg := getFileConfig()
	if cfg == nil || len(cfg.SelectedModels) == 0 {
		return nil
	}

	type resolvedModel struct {
		providerName string
		modelName    string
		baseURL      string
		apiKey       string
	}

	modelIndex := make(map[string]*resolvedModel)
	for _, p := range cfg.Providers {
		for _, g := range p.Groups {
			for _, m := range g.Models {
				baseURL := m.BaseURL
				if baseURL == "" {
					baseURL = g.BaseURL
				}
				modelIndex[m.ID] = &resolvedModel{
					providerName: p.Name,
					modelName:    m.Name,
					baseURL:      baseURL,
					apiKey:       g.APIKey,
				}
			}
		}
	}

	out := map[string]any{}
	for _, sel := range cfg.SelectedModels {
		rm, ok := modelIndex[sel.ModelID]
		if !ok {
			continue
		}
		entry := map[string]any{
			"source":   strings.ToLower(strings.TrimSpace(rm.providerName)),
			"model":    rm.modelName,
			"base_url": rm.baseURL,
			"api_key":  rm.apiKey,
		}
		switch strings.ToLower(strings.TrimSpace(sel.ModelType)) {
		case "llm", "llm-chat":
			out["llm"] = entry
		case "llm-evo", "llm2":
			out["evo_llm"] = entry
		case "embedding", "embed":
			out["embed_main"] = entry
		case "rerank", "reranker":
			out["reranker"] = entry
		}
	}
	if _, ok := out["evo_llm"]; !ok {
		if cfg, ok := out["llm"]; ok {
			out["evo_llm"] = cfg
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
