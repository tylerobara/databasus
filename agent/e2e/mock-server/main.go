package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
)

type server struct {
	mu         sync.RWMutex
	version    string
	binaryPath string
}

func main() {
	version := "v2.0.0"
	binaryPath := "/artifacts/agent-v2"
	port := "4050"

	s := &server{version: version, binaryPath: binaryPath}

	http.HandleFunc("/api/v1/system/version", s.handleVersion)
	http.HandleFunc("/api/v1/system/agent", s.handleAgentDownload)
	http.HandleFunc("/mock/set-version", s.handleSetVersion)
	http.HandleFunc("/mock/set-binary-path", s.handleSetBinaryPath)
	http.HandleFunc("/health", s.handleHealth)

	addr := ":" + port
	log.Printf("Mock server starting on %s (version=%s, binary=%s)", addr, version, binaryPath)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func (s *server) handleVersion(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	v := s.version
	s.mu.RUnlock()

	log.Printf("GET /api/v1/system/version -> %s", v)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"version": v})
}

func (s *server) handleAgentDownload(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	path := s.binaryPath
	s.mu.RUnlock()

	log.Printf("GET /api/v1/system/agent (arch=%s) -> serving %s", r.URL.Query().Get("arch"), path)

	http.ServeFile(w, r, path)
}

func (s *server) handleSetVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	s.version = body.Version
	s.mu.Unlock()

	log.Printf("POST /mock/set-version -> %s", body.Version)

	_, _ = fmt.Fprintf(w, "version set to %s", body.Version)
}

func (s *server) handleSetBinaryPath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		BinaryPath string `json:"binaryPath"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	s.binaryPath = body.BinaryPath
	s.mu.Unlock()

	log.Printf("POST /mock/set-binary-path -> %s", body.BinaryPath)

	_, _ = fmt.Fprintf(w, "binary path set to %s", body.BinaryPath)
}

func (s *server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
