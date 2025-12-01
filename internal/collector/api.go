package collector

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
)

type API struct {
	db *DB
}

func NewAPI(db *DB) *API {
	return &API{db: db}
}

func (api *API) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/hosts", api.handleHosts)
	mux.HandleFunc("/api/v1/hosts/", api.handleHost)
	mux.HandleFunc("/api/v1/stats", api.handleStats)
	mux.HandleFunc("/api/v1/health", api.handleHealth)
}

func (api *API) handleHosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hosts, err := api.db.GetAllHosts()
	if err != nil {
		log.Printf("Error getting hosts: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"hosts": hosts,
		"count": len(hosts),
	})
}

func (api *API) handleHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hostname := r.URL.Path[len("/api/v1/hosts/"):]
	if hostname == "" {
		http.Error(w, "Hostname required", http.StatusBadRequest)
		return
	}

	host, err := api.db.GetHost(hostname)
	if err == sql.ErrNoRows {
		http.Error(w, "Host not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("Error getting host %s: %v", hostname, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	usage, err := api.db.GetHostUsage(hostname, limit)
	if err != nil {
		log.Printf("Error getting usage for %s: %v", hostname, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"host":  host,
		"usage": usage,
	})
}

func (api *API) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats, err := api.db.GetClusterStats()
	if err != nil {
		log.Printf("Error getting cluster stats: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

func (api *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := api.db.conn.Ping(); err != nil {
		http.Error(w, fmt.Sprintf("Database unhealthy: %v", err), http.StatusServiceUnavailable)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "healthy",
		"database": "connected",
	})
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}
