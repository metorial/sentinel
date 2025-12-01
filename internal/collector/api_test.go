package collector

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/metorial/fleet/node-manager/internal/models"
)

func TestHandleHealth(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	api := NewAPI(db)
	mux := http.NewServeMux()
	api.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("Expected status healthy, got %v", response["status"])
	}
}

func TestHandleHosts(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	host1 := &models.Host{
		Hostname:          "test-host-1",
		IP:                "192.168.1.100",
		UptimeSeconds:     3600,
		CPUCores:          4,
		TotalMemoryBytes:  8589934592,
		TotalStorageBytes: 107374182400,
		LastSeen:          time.Now(),
		Online:            true,
	}

	host2 := &models.Host{
		Hostname:          "test-host-2",
		IP:                "192.168.1.101",
		UptimeSeconds:     7200,
		CPUCores:          8,
		TotalMemoryBytes:  17179869184,
		TotalStorageBytes: 214748364800,
		LastSeen:          time.Now(),
		Online:            true,
	}

	if _, err := db.UpsertHost(host1); err != nil {
		t.Fatalf("Failed to insert host1: %v", err)
	}

	if _, err := db.UpsertHost(host2); err != nil {
		t.Fatalf("Failed to insert host2: %v", err)
	}

	api := NewAPI(db)
	mux := http.NewServeMux()
	api.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/hosts", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	count := int(response["count"].(float64))
	if count != 2 {
		t.Errorf("Expected 2 hosts, got %d", count)
	}
}

func TestHandleHost(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	host := &models.Host{
		Hostname:          "test-host",
		IP:                "192.168.1.100",
		UptimeSeconds:     3600,
		CPUCores:          4,
		TotalMemoryBytes:  8589934592,
		TotalStorageBytes: 107374182400,
		LastSeen:          time.Now(),
		Online:            true,
	}

	hostID, err := db.UpsertHost(host)
	if err != nil {
		t.Fatalf("Failed to insert host: %v", err)
	}

	usage := &models.HostUsage{
		HostID:           hostID,
		Timestamp:        time.Now(),
		CPUPercent:       45.5,
		UsedMemoryBytes:  4294967296,
		UsedStorageBytes: 53687091200,
	}

	if err := db.InsertUsage(usage); err != nil {
		t.Fatalf("Failed to insert usage: %v", err)
	}

	api := NewAPI(db)
	mux := http.NewServeMux()
	api.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/hosts/test-host", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	hostData := response["host"].(map[string]interface{})
	if hostData["hostname"] != "test-host" {
		t.Errorf("Expected hostname test-host, got %v", hostData["hostname"])
	}

	usageData := response["usage"].([]interface{})
	if len(usageData) != 1 {
		t.Errorf("Expected 1 usage record, got %d", len(usageData))
	}
}

func TestHandleHostNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	api := NewAPI(db)
	mux := http.NewServeMux()
	api.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/hosts/nonexistent", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandleStats(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	host1 := &models.Host{
		Hostname:          "test-host-1",
		IP:                "192.168.1.100",
		UptimeSeconds:     3600,
		CPUCores:          4,
		TotalMemoryBytes:  8589934592,
		TotalStorageBytes: 107374182400,
		LastSeen:          time.Now(),
		Online:            true,
	}

	host2 := &models.Host{
		Hostname:          "test-host-2",
		IP:                "192.168.1.101",
		UptimeSeconds:     7200,
		CPUCores:          8,
		TotalMemoryBytes:  17179869184,
		TotalStorageBytes: 214748364800,
		LastSeen:          time.Now().Add(-5 * time.Minute),
		Online:            false,
	}

	if _, err := db.UpsertHost(host1); err != nil {
		t.Fatalf("Failed to insert host1: %v", err)
	}

	if _, err := db.UpsertHost(host2); err != nil {
		t.Fatalf("Failed to insert host2: %v", err)
	}

	api := NewAPI(db)
	mux := http.NewServeMux()
	api.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	totalHosts := int(response["total_hosts"].(float64))
	if totalHosts != 2 {
		t.Errorf("Expected 2 total hosts, got %d", totalHosts)
	}

	onlineHosts := int(response["online_hosts"].(float64))
	if onlineHosts != 1 {
		t.Errorf("Expected 1 online host, got %d", onlineHosts)
	}

	offlineHosts := int(response["offline_hosts"].(float64))
	if offlineHosts != 1 {
		t.Errorf("Expected 1 offline host, got %d", offlineHosts)
	}

	totalCPUCores := int(response["total_cpu_cores"].(float64))
	if totalCPUCores != 4 {
		t.Errorf("Expected 4 CPU cores (only online), got %d", totalCPUCores)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	api := NewAPI(db)
	mux := http.NewServeMux()
	api.RegisterRoutes(mux)

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/hosts"},
		{http.MethodDelete, "/api/v1/hosts/test"},
		{http.MethodPut, "/api/v1/stats"},
		{http.MethodPost, "/api/v1/health"},
	}

	for _, tt := range tests {
		t.Run(tt.method+"_"+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("Expected status 405, got %d", w.Code)
			}
		})
	}
}
