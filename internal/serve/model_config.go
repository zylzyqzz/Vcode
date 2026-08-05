package serve

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"vcode/internal/config"
)

var modelConfigNameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{1,48}$`)

type modelConfigView struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	BaseURL   string   `json:"base_url"`
	Models    []string `json:"models"`
	KeySet    bool     `json:"key_set"`
	IsDefault bool     `json:"is_default"`
}

type modelConfigRequest struct {
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	APIKey  string `json:"api_key"`
	Default bool   `json:"default"`
}

func (s *Server) modelConfigs(w http.ResponseWriter, _ *http.Request) {
	cfg, err := config.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]modelConfigView, 0, len(cfg.Providers))
	for _, p := range cfg.Providers {
		models := p.ModelList()
		out = append(out, modelConfigView{
			Name: p.Name, Kind: p.Kind, BaseURL: p.BaseURL, Models: models,
			KeySet: p.APIKey() != "", IsDefault: cfg.DefaultModel == p.Name || cfg.DefaultModel == p.Name+"/"+p.DefaultModel(),
		})
	}
	writeModelJSON(w, http.StatusOK, map[string]any{"default": cfg.DefaultModel, "providers": out})
}

func (s *Server) addModelConfig(w http.ResponseWriter, r *http.Request) {
	var req modelConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	req.Name = strings.ToLower(strings.TrimSpace(req.Name))
	req.BaseURL = strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	req.Model = strings.TrimSpace(req.Model)
	if !modelConfigNameRE.MatchString(req.Name) {
		http.Error(w, "name must use lowercase letters, numbers, and hyphens", http.StatusBadRequest)
		return
	}
	if req.BaseURL == "" || (!strings.HasPrefix(req.BaseURL, "https://") && !strings.HasPrefix(req.BaseURL, "http://")) {
		http.Error(w, "base_url must start with http:// or https://", http.StatusBadRequest)
		return
	}
	if req.Model == "" {
		http.Error(w, "model is required", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.APIKey) == "" {
		http.Error(w, "api_key is required", http.StatusBadRequest)
		return
	}

	path := config.UserConfigPath()
	edit := config.LoadForEdit(path)
	if edit == nil {
		http.Error(w, "configuration is not available", http.StatusInternalServerError)
		return
	}
	envName := "VCODE_" + strings.ToUpper(strings.ReplaceAll(req.Name, "-", "_")) + "_API_KEY"
	entry := config.ProviderEntry{Name: req.Name, Kind: "openai", BaseURL: req.BaseURL, Model: req.Model, Models: []string{req.Model}, Default: req.Model, APIKeyEnv: envName, ContextWindow: 1_000_000}
	if err := edit.UpsertProvider(entry); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Default {
		if err := edit.SetDefaultModel(req.Name); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	if err := edit.SaveTo(path); err != nil {
		http.Error(w, fmt.Sprintf("save config: %v", err), http.StatusInternalServerError)
		return
	}
	if _, err := config.SetCredential(envName, strings.TrimSpace(req.APIKey)); err != nil {
		http.Error(w, fmt.Sprintf("save credential: %v", err), http.StatusInternalServerError)
		return
	}
	if req.Default {
		if err := s.switchModel(r.Context(), req.Name); err != nil {
			http.Error(w, fmt.Sprintf("model saved but switch failed: %v", err), http.StatusBadGateway)
			return
		}
	}
	writeModelJSON(w, http.StatusCreated, map[string]any{"name": req.Name, "default": req.Default})
}

func (s *Server) setDefaultModel(w http.ResponseWriter, r *http.Request) {
	var req struct{ Name string `json:"name"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	path := config.UserConfigPath()
	edit := config.LoadForEdit(path)
	if edit == nil {
		http.Error(w, "configuration is not available", http.StatusInternalServerError)
		return
	}
	if err := edit.SetDefaultModel(strings.TrimSpace(req.Name)); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := edit.SaveTo(path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.switchModel(r.Context(), strings.TrimSpace(req.Name)); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeModelJSON(w, http.StatusOK, map[string]any{"default": strings.TrimSpace(req.Name)})
}

func writeModelJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
