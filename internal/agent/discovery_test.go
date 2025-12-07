package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDiscoverCommander(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health/service/sentinel-controller" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		response := []map[string]interface{}{
			{
				"Node": map[string]interface{}{
					"Address": "10.0.0.1",
				},
				"Service": map[string]interface{}{
					"Address": "10.0.0.2",
					"Port":    9090,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	sd, err := NewServiceDiscovery(server.URL[7:])
	if err != nil {
		t.Fatalf("Failed to create service discovery: %v", err)
	}

	addr, err := sd.DiscoverCommander()
	if err != nil {
		t.Fatalf("Failed to discover collector: %v", err)
	}

	expected := "10.0.0.2:9090"
	if addr != expected {
		t.Errorf("Expected address %s, got %s", expected, addr)
	}
}

func TestDiscoverCommanderNoServices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{})
	}))
	defer server.Close()

	sd, err := NewServiceDiscovery(server.URL[7:])
	if err != nil {
		t.Fatalf("Failed to create service discovery: %v", err)
	}

	_, err = sd.DiscoverCommander()
	if err == nil {
		t.Error("Expected error when no services found")
	}
}

func TestDiscoverCommanderUsesNodeAddress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := []map[string]interface{}{
			{
				"Node": map[string]interface{}{
					"Address": "10.0.0.1",
				},
				"Service": map[string]interface{}{
					"Address": "",
					"Port":    9090,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	sd, err := NewServiceDiscovery(server.URL[7:])
	if err != nil {
		t.Fatalf("Failed to create service discovery: %v", err)
	}

	addr, err := sd.DiscoverCommander()
	if err != nil {
		t.Fatalf("Failed to discover collector: %v", err)
	}

	expected := "10.0.0.1:9090"
	if addr != expected {
		t.Errorf("Expected address %s (node address), got %s", expected, addr)
	}
}

func TestWatchCommander(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		response := []map[string]interface{}{
			{
				"Node": map[string]interface{}{
					"Address": "10.0.0.1",
				},
				"Service": map[string]interface{}{
					"Address": "10.0.0.2",
					"Port":    9090,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	sd, err := NewServiceDiscovery(server.URL[7:])
	if err != nil {
		t.Fatalf("Failed to create service discovery: %v", err)
	}

	addrChan := sd.WatchCommander()

	select {
	case addr := <-addrChan:
		expected := "10.0.0.2:9090"
		if addr != expected {
			t.Errorf("Expected address %s, got %s", expected, addr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for collector address")
	}
}
