package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/health" {
			t.Errorf("Expected path /api/v1/health, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "healthy",
			"database": "connected",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	data, err := client.Health()
	if err != nil {
		t.Fatalf("Health() error: %v", err)
	}

	if data["status"] != "healthy" {
		t.Errorf("Expected status healthy, got %v", data["status"])
	}
}

func TestClientListHosts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/hosts" {
			t.Errorf("Expected path /api/v1/hosts, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"hosts": []map[string]interface{}{
				{"hostname": "test-host", "ip": "192.168.1.100"},
			},
			"count": 1,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	data, err := client.ListHosts()
	if err != nil {
		t.Fatalf("ListHosts() error: %v", err)
	}

	count := int(data["count"].(float64))
	if count != 1 {
		t.Errorf("Expected count 1, got %d", count)
	}
}

func TestClientGetHost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/hosts/test-host" {
			t.Errorf("Expected path /api/v1/hosts/test-host, got %s", r.URL.Path)
		}

		limit := r.URL.Query().Get("limit")
		if limit != "50" {
			t.Errorf("Expected limit=50, got %s", limit)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"host": map[string]interface{}{
				"hostname": "test-host",
				"ip":       "192.168.1.100",
			},
			"usage": []interface{}{},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	data, err := client.GetHost("test-host", 50)
	if err != nil {
		t.Fatalf("GetHost() error: %v", err)
	}

	host := data["host"].(map[string]interface{})
	if host["hostname"] != "test-host" {
		t.Errorf("Expected hostname test-host, got %v", host["hostname"])
	}
}

func TestClientGetStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/stats" {
			t.Errorf("Expected path /api/v1/stats, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total_hosts":  10,
			"online_hosts": 8,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	data, err := client.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error: %v", err)
	}

	totalHosts := int(data["total_hosts"].(float64))
	if totalHosts != 10 {
		t.Errorf("Expected total_hosts 10, got %d", totalHosts)
	}
}

func TestClientErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not found"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Health()
	if err == nil {
		t.Error("Expected error for 404 response")
	}
}
