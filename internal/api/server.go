package api

import (
	"context"
	"docs_organiser/internal/config"
	"docs_organiser/internal/pipeline"
	"docs_organiser/internal/storage"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type Server struct {
	cfg        *config.Config
	pipeline   *pipeline.Pipeline
	store      storage.Store
	mu         sync.RWMutex
	currentCtx context.Context
	cancelFunc context.CancelFunc
}

func NewServer(cfg *config.Config, p *pipeline.Pipeline, store storage.Store) *Server {
	return &Server{
		cfg:      cfg,
		pipeline: p,
		store:    store,
	}
}

type StartRequest struct {
	SourceDir string `json:"src"`
	DestDir   string `json:"dst"`
	Model     string `json:"model"`
	ModelURL  string `json:"model_url"`
	Workers   int    `json:"workers"`
	Limit     int    `json:"limit"`
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /v1/pipeline/start", s.handleStart)
	mux.HandleFunc("POST /v1/pipeline/stop", s.handleStop)
	mux.HandleFunc("POST /v1/pipeline/pause", s.handlePause)
	mux.HandleFunc("POST /v1/pipeline/resume", s.handleResume)
	mux.HandleFunc("GET /v1/pipeline/status", s.handleStatus)
	mux.HandleFunc("GET /v1/config", s.handleGetConfig)

	mux.HandleFunc("GET /v1/models", s.handleGetModels)
	mux.HandleFunc("POST /v1/models", s.handleAddModel)
	mux.HandleFunc("DELETE /v1/models", s.handleRemoveModel)
	mux.HandleFunc("POST /v1/models/default", s.handleSetDefaultModel)

	// Serve React UI
	uiDir := "ui/dist"
	fileServer := http.FileServer(http.Dir(uiDir))
	mux.Handle("/", http.StripPrefix("/", fileServer))

	server := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", s.cfg.ServerPort),
		Handler: mux,
	}

	fmt.Printf("[+] App Server started at http://localhost:%d\n", s.cfg.ServerPort)
	return server.ListenAndServe()
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancelFunc != nil {
		http.Error(w, "Pipeline already running", http.StatusConflict)
		return
	}

	var req StartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.SourceDir != "" {
		s.pipeline.SourceDir = req.SourceDir
	}
	if req.DestDir != "" {
		s.pipeline.DestDir = req.DestDir
	}
	if req.Model != "" {
		s.pipeline.SetModel(req.Model, req.ModelURL)
	}
	if req.Workers > 0 {
		s.pipeline.Workers = req.Workers
	}
	if req.Limit > 0 {
		s.pipeline.ExtractLimit = req.Limit
	}

	// Persist changes if any were made via request
	if req.SourceDir != "" || req.DestDir != "" || req.Model != "" || req.Workers > 0 || req.Limit > 0 {
		_ = s.store.Save("user_settings", s.cfg)
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.currentCtx = ctx
	s.cancelFunc = cancel

	go func() {
		defer func() {
			s.mu.Lock()
			s.cancelFunc = nil
			s.mu.Unlock()
		}()
		_ = s.pipeline.Run(ctx)
	}()

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancelFunc == nil {
		http.Error(w, "Pipeline not running", http.StatusNotFound)
		return
	}

	s.cancelFunc()
	s.cancelFunc = nil

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

func (s *Server) handlePause(w http.ResponseWriter, r *http.Request) {
	s.pipeline.Pause()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "paused"})
}

func (s *Server) handleResume(w http.ResponseWriter, r *http.Request) {
	s.pipeline.Resume()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "resumed"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	summary := s.pipeline.GetSummary()
	s.mu.RLock()
	isRunning := s.cancelFunc != nil
	s.mu.RUnlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"total":      s.pipeline.TotalFiles,
		"processed":  s.pipeline.ProcessedFiles,
		"failed":     s.pipeline.FailedFiles,
		"summary":    summary,
		"is_running": isRunning,
		"is_paused":  s.pipeline.IsPaused(),
	})
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(s.cfg)
}

func (s *Server) handleGetModels(w http.ResponseWriter, r *http.Request) {
	type modelInfo struct {
		Name      string `json:"name"`
		URL       string `json:"url"`
		Status    string `json:"status"`
		IsDefault bool   `json:"is_default"`
	}

	var models []modelInfo
	for _, m := range s.cfg.AllowedModels {
		status := "Offline"
		available, err := s.pipeline.AI.GetAvailableModelsForURL(r.Context(), m.URL)
		if err == nil {
			for _, avail := range available {
				if m.Name == avail {
					status = "Active"
					break
				}
			}
		}
		models = append(models, modelInfo{
			Name:      m.Name,
			URL:       m.URL,
			Status:    status,
			IsDefault: m.Name == s.cfg.DefaultModelName,
		})
	}

	json.NewEncoder(w).Encode(models)
}

func (s *Server) handleAddModel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if exists
	exists := false
	for _, m := range s.cfg.AllowedModels {
		if m.Name == req.Name {
			exists = true
			break
		}
	}

	if !exists {
		s.cfg.AllowedModels = append(s.cfg.AllowedModels, config.ModelDefinition{
			Name: req.Name,
			URL:  req.URL,
		})
		s.pipeline.AI.SetAllowedModels(s.cfg.AllowedModels)
		_ = s.store.Save("user_settings", s.cfg)
	}

	// If it's the first model, set it as default
	if len(s.cfg.AllowedModels) == 1 {
		s.cfg.DefaultModelName = s.cfg.AllowedModels[0].Name
		s.pipeline.AI.SetDefaultModel(s.cfg.DefaultModelName)
		_ = s.store.Save("user_settings", s.cfg)
	}

	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleSetDefaultModel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Verify model exists
	found := false
	for _, m := range s.cfg.AllowedModels {
		if m.Name == req.Name {
			found = true
			break
		}
	}

	if !found {
		http.Error(w, "model not found in pool", http.StatusNotFound)
		return
	}

	s.mu.Lock()
	s.cfg.DefaultModelName = req.Name
	s.pipeline.AI.SetDefaultModel(req.Name)
	_ = s.store.Save("user_settings", s.cfg)
	s.mu.Unlock()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "default model updated"})
}

func (s *Server) handleRemoveModel(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "missing name parameter", http.StatusBadRequest)
		return
	}

	var newList []config.ModelDefinition
	for _, m := range s.cfg.AllowedModels {
		if m.Name != name {
			newList = append(newList, m)
		}
	}
	s.cfg.AllowedModels = newList
	if s.cfg.DefaultModelName == name {
		if len(newList) > 0 {
			s.cfg.DefaultModelName = newList[0].Name
		} else {
			s.cfg.DefaultModelName = ""
		}
		s.pipeline.AI.SetDefaultModel(s.cfg.DefaultModelName)
	}
	s.pipeline.AI.SetAllowedModels(s.cfg.AllowedModels)
	_ = s.store.Save("user_settings", s.cfg)

	w.WriteHeader(http.StatusOK)
}
