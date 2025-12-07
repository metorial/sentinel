package agent

import (
	"strings"
	"testing"
)

func TestNewMetricsCollector(t *testing.T) {
	mc, err := NewMetricsCollector()
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}

	if mc.hostname == "" {
		t.Error("Expected non-empty hostname")
	}

	if mc.ip == "" {
		t.Error("Expected non-empty IP")
	}
}

func TestCollect(t *testing.T) {
	mc, err := NewMetricsCollector()
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}

	metrics, err := mc.Collect()
	if err != nil {
		// Skip test if CPU metrics not available (CGO disabled or platform limitation)
		if strings.Contains(err.Error(), "not implemented yet") {
			t.Skip("Skipping test: CPU metrics not available without CGO")
		}
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	if metrics == nil {
		t.Fatal("Expected non-nil metrics")
	}

	if metrics.Hostname == "" {
		t.Error("Expected non-empty hostname")
	}

	if metrics.Ip == "" {
		t.Error("Expected non-empty IP")
	}

	if metrics.Timestamp == 0 {
		t.Error("Expected non-zero timestamp")
	}

	if metrics.Info == nil {
		t.Fatal("Expected non-nil info")
	}

	if metrics.Info.CpuCores <= 0 {
		t.Error("Expected positive CPU cores")
	}

	if metrics.Info.TotalMemoryBytes <= 0 {
		t.Error("Expected positive total memory")
	}

	if metrics.Info.TotalStorageBytes <= 0 {
		t.Error("Expected positive total storage")
	}

	if metrics.Usage == nil {
		t.Fatal("Expected non-nil usage")
	}

	if metrics.Usage.CpuPercent < 0 || metrics.Usage.CpuPercent > 100 {
		t.Errorf("Expected CPU percent in range [0,100], got %f", metrics.Usage.CpuPercent)
	}

	if metrics.Usage.UsedMemoryBytes < 0 {
		t.Error("Expected non-negative used memory")
	}

	if metrics.Usage.UsedStorageBytes < 0 {
		t.Error("Expected non-negative used storage")
	}
}

func TestCollectInfo(t *testing.T) {
	mc, err := NewMetricsCollector()
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}

	info, err := mc.collectInfo()
	if err != nil {
		t.Fatalf("Failed to collect info: %v", err)
	}

	if info.UptimeSeconds < 0 {
		t.Error("Expected non-negative uptime")
	}

	if info.CpuCores <= 0 {
		t.Error("Expected positive CPU cores")
	}

	if info.TotalMemoryBytes <= 0 {
		t.Error("Expected positive total memory")
	}

	if info.TotalStorageBytes <= 0 {
		t.Error("Expected positive total storage")
	}
}

func TestCollectUsage(t *testing.T) {
	mc, err := NewMetricsCollector()
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}

	usage, err := mc.collectUsage()
	if err != nil {
		// Skip test if CPU metrics not available (CGO disabled or platform limitation)
		if strings.Contains(err.Error(), "not implemented yet") {
			t.Skip("Skipping test: CPU metrics not available without CGO")
		}
		t.Fatalf("Failed to collect usage: %v", err)
	}

	if usage.CpuPercent < 0 {
		t.Error("Expected non-negative CPU percent")
	}

	if usage.UsedMemoryBytes < 0 {
		t.Error("Expected non-negative used memory")
	}

	if usage.UsedStorageBytes < 0 {
		t.Error("Expected non-negative used storage")
	}
}
