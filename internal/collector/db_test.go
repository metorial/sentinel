package collector

import (
	"os"
	"testing"
	"time"

	"github.com/metorial/fleet/node-manager/internal/models"
)

func TestNewDB(t *testing.T) {
	dbPath := t.TempDir() + "/test.db"
	defer os.Remove(dbPath)

	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	if db.conn == nil {
		t.Fatal("Database connection is nil")
	}
}

func TestUpsertHost(t *testing.T) {
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

	id, err := db.UpsertHost(host)
	if err != nil {
		t.Fatalf("Failed to insert host: %v", err)
	}

	if id == 0 {
		t.Fatal("Expected non-zero ID")
	}

	host.IP = "192.168.1.101"
	host.CPUCores = 8

	id2, err := db.UpsertHost(host)
	if err != nil {
		t.Fatalf("Failed to update host: %v", err)
	}

	if id != id2 {
		t.Errorf("Expected same ID after upsert, got %d and %d", id, id2)
	}

	var ip string
	var cores int32
	err = db.conn.QueryRow("SELECT ip, cpu_cores FROM hosts WHERE id = ?", id).Scan(&ip, &cores)
	if err != nil {
		t.Fatalf("Failed to query host: %v", err)
	}

	if ip != "192.168.1.101" {
		t.Errorf("Expected IP 192.168.1.101, got %s", ip)
	}

	if cores != 8 {
		t.Errorf("Expected 8 cores, got %d", cores)
	}
}

func TestInsertUsage(t *testing.T) {
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

	var count int
	err = db.conn.QueryRow("SELECT COUNT(*) FROM host_usage WHERE host_id = ?", hostID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query usage count: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 usage record, got %d", count)
	}
}

func TestMarkInactive(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now()
	oldHost := &models.Host{
		Hostname:          "old-host",
		IP:                "192.168.1.100",
		UptimeSeconds:     3600,
		CPUCores:          4,
		TotalMemoryBytes:  8589934592,
		TotalStorageBytes: 107374182400,
		LastSeen:          now.Add(-5 * time.Minute),
		Online:            true,
	}

	activeHost := &models.Host{
		Hostname:          "active-host",
		IP:                "192.168.1.101",
		UptimeSeconds:     3600,
		CPUCores:          4,
		TotalMemoryBytes:  8589934592,
		TotalStorageBytes: 107374182400,
		LastSeen:          now,
		Online:            true,
	}

	if _, err := db.UpsertHost(oldHost); err != nil {
		t.Fatalf("Failed to insert old host: %v", err)
	}

	if _, err := db.UpsertHost(activeHost); err != nil {
		t.Fatalf("Failed to insert active host: %v", err)
	}

	if err := db.MarkInactive(2 * time.Minute); err != nil {
		t.Fatalf("Failed to mark inactive: %v", err)
	}

	var onlineCount int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM hosts WHERE online = 1").Scan(&onlineCount)
	if err != nil {
		t.Fatalf("Failed to query online count: %v", err)
	}

	if onlineCount != 1 {
		t.Errorf("Expected 1 online host, got %d", onlineCount)
	}
}

func TestCleanupOldUsage(t *testing.T) {
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

	now := time.Now()
	oldUsage := &models.HostUsage{
		HostID:           hostID,
		Timestamp:        now.Add(-10 * 24 * time.Hour),
		CPUPercent:       45.5,
		UsedMemoryBytes:  4294967296,
		UsedStorageBytes: 53687091200,
	}

	recentUsage := &models.HostUsage{
		HostID:           hostID,
		Timestamp:        now,
		CPUPercent:       50.0,
		UsedMemoryBytes:  4294967296,
		UsedStorageBytes: 53687091200,
	}

	if err := db.InsertUsage(oldUsage); err != nil {
		t.Fatalf("Failed to insert old usage: %v", err)
	}

	if err := db.InsertUsage(recentUsage); err != nil {
		t.Fatalf("Failed to insert recent usage: %v", err)
	}

	if err := db.CleanupOldUsage(7 * 24 * time.Hour); err != nil {
		t.Fatalf("Failed to cleanup old usage: %v", err)
	}

	var count int
	err = db.conn.QueryRow("SELECT COUNT(*) FROM host_usage WHERE host_id = ?", hostID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query usage count: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 usage record after cleanup, got %d", count)
	}
}

func setupTestDB(t *testing.T) *DB {
	t.Helper()
	dbPath := t.TempDir() + "/test.db"
	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	return db
}
