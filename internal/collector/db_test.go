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

func TestCreateScript(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	script := &models.Script{
		ID:         "test-script-id",
		Name:       "test-script",
		Content:    "#!/bin/bash\necho 'hello'",
		SHA256Hash: "abc123",
		CreatedAt:  time.Now(),
	}

	if err := db.CreateScript(script); err != nil {
		t.Fatalf("Failed to create script: %v", err)
	}

	retrieved, err := db.GetScript(script.ID)
	if err != nil {
		t.Fatalf("Failed to get script: %v", err)
	}

	if retrieved.Name != script.Name {
		t.Errorf("Expected name %s, got %s", script.Name, retrieved.Name)
	}

	if retrieved.Content != script.Content {
		t.Errorf("Expected content %s, got %s", script.Content, retrieved.Content)
	}
}

func TestGetAllScripts(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	scripts := []*models.Script{
		{
			ID:         "script-1",
			Name:       "script-one",
			Content:    "#!/bin/bash\necho '1'",
			SHA256Hash: "hash1",
			CreatedAt:  time.Now(),
		},
		{
			ID:         "script-2",
			Name:       "script-two",
			Content:    "#!/bin/bash\necho '2'",
			SHA256Hash: "hash2",
			CreatedAt:  time.Now().Add(time.Second),
		},
	}

	for _, s := range scripts {
		if err := db.CreateScript(s); err != nil {
			t.Fatalf("Failed to create script: %v", err)
		}
	}

	retrieved, err := db.GetAllScripts()
	if err != nil {
		t.Fatalf("Failed to get all scripts: %v", err)
	}

	if len(retrieved) != 2 {
		t.Errorf("Expected 2 scripts, got %d", len(retrieved))
	}
}

func TestDeleteScript(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	script := &models.Script{
		ID:         "test-delete",
		Name:       "test",
		Content:    "test",
		SHA256Hash: "hash",
		CreatedAt:  time.Now(),
	}

	if err := db.CreateScript(script); err != nil {
		t.Fatalf("Failed to create script: %v", err)
	}

	if err := db.DeleteScript(script.ID); err != nil {
		t.Fatalf("Failed to delete script: %v", err)
	}

	_, err := db.GetScript(script.ID)
	if err == nil {
		t.Error("Expected error when getting deleted script")
	}
}

func TestRecordScriptExecution(t *testing.T) {
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

	script := &models.Script{
		ID:         "test-script",
		Name:       "test",
		Content:    "test",
		SHA256Hash: "hash",
		CreatedAt:  time.Now(),
	}

	if err := db.CreateScript(script); err != nil {
		t.Fatalf("Failed to create script: %v", err)
	}

	exec := &models.ScriptExecution{
		ScriptID:   script.ID,
		HostID:     hostID,
		SHA256Hash: script.SHA256Hash,
		ExitCode:   0,
		Stdout:     "success",
		Stderr:     "",
		ExecutedAt: time.Now(),
	}

	if err := db.RecordScriptExecution(exec); err != nil {
		t.Fatalf("Failed to record execution: %v", err)
	}

	executions, err := db.GetScriptExecutions(script.ID)
	if err != nil {
		t.Fatalf("Failed to get executions: %v", err)
	}

	if len(executions) != 1 {
		t.Errorf("Expected 1 execution, got %d", len(executions))
	}

	if executions[0].ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", executions[0].ExitCode)
	}

	if executions[0].Stdout != "success" {
		t.Errorf("Expected stdout 'success', got %s", executions[0].Stdout)
	}
}

func TestHasScriptExecuted(t *testing.T) {
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

	script := &models.Script{
		ID:         "test-script",
		Name:       "test",
		Content:    "test",
		SHA256Hash: "testhash",
		CreatedAt:  time.Now(),
	}

	if err := db.CreateScript(script); err != nil {
		t.Fatalf("Failed to create script: %v", err)
	}

	hasExecuted, err := db.HasScriptExecuted(host.Hostname, script.SHA256Hash)
	if err != nil {
		t.Fatalf("Failed to check execution: %v", err)
	}

	if hasExecuted {
		t.Error("Expected script to not have been executed")
	}

	exec := &models.ScriptExecution{
		ScriptID:   script.ID,
		HostID:     hostID,
		SHA256Hash: script.SHA256Hash,
		ExitCode:   0,
		Stdout:     "",
		Stderr:     "",
		ExecutedAt: time.Now(),
	}

	if err := db.RecordScriptExecution(exec); err != nil {
		t.Fatalf("Failed to record execution: %v", err)
	}

	hasExecuted, err = db.HasScriptExecuted(host.Hostname, script.SHA256Hash)
	if err != nil {
		t.Fatalf("Failed to check execution: %v", err)
	}

	if !hasExecuted {
		t.Error("Expected script to have been executed")
	}
}

func TestTagManagement(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tagID, err := db.CreateTag("production")
	if err != nil {
		t.Fatalf("Failed to create tag: %v", err)
	}

	if tagID == 0 {
		t.Error("Expected non-zero tag ID")
	}

	tags, err := db.GetAllTags()
	if err != nil {
		t.Fatalf("Failed to get tags: %v", err)
	}

	if len(tags) != 1 {
		t.Errorf("Expected 1 tag, got %d", len(tags))
	}

	if tags[0].Name != "production" {
		t.Errorf("Expected tag name 'production', got %s", tags[0].Name)
	}
}

func TestGetOrCreateTag(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	id1, err := db.GetOrCreateTag("test-tag")
	if err != nil {
		t.Fatalf("Failed to create tag: %v", err)
	}

	id2, err := db.GetOrCreateTag("test-tag")
	if err != nil {
		t.Fatalf("Failed to get tag: %v", err)
	}

	if id1 != id2 {
		t.Errorf("Expected same tag ID, got %d and %d", id1, id2)
	}
}

func TestHostTagManagement(t *testing.T) {
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

	if _, err := db.UpsertHost(host); err != nil {
		t.Fatalf("Failed to insert host: %v", err)
	}

	if err := db.AddHostTag(host.Hostname, "production"); err != nil {
		t.Fatalf("Failed to add tag: %v", err)
	}

	if err := db.AddHostTag(host.Hostname, "web-server"); err != nil {
		t.Fatalf("Failed to add second tag: %v", err)
	}

	tags, err := db.GetHostTags(host.Hostname)
	if err != nil {
		t.Fatalf("Failed to get host tags: %v", err)
	}

	if len(tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(tags))
	}

	if err := db.RemoveHostTag(host.Hostname, "production"); err != nil {
		t.Fatalf("Failed to remove tag: %v", err)
	}

	tags, err = db.GetHostTags(host.Hostname)
	if err != nil {
		t.Fatalf("Failed to get host tags: %v", err)
	}

	if len(tags) != 1 {
		t.Errorf("Expected 1 tag after removal, got %d", len(tags))
	}
}

func TestGetHostsByTags(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	hosts := []*models.Host{
		{
			Hostname:          "prod-host-1",
			IP:                "192.168.1.100",
			UptimeSeconds:     3600,
			CPUCores:          4,
			TotalMemoryBytes:  8589934592,
			TotalStorageBytes: 107374182400,
			LastSeen:          time.Now(),
			Online:            true,
		},
		{
			Hostname:          "prod-host-2",
			IP:                "192.168.1.101",
			UptimeSeconds:     3600,
			CPUCores:          4,
			TotalMemoryBytes:  8589934592,
			TotalStorageBytes: 107374182400,
			LastSeen:          time.Now(),
			Online:            true,
		},
		{
			Hostname:          "dev-host-1",
			IP:                "192.168.1.102",
			UptimeSeconds:     3600,
			CPUCores:          4,
			TotalMemoryBytes:  8589934592,
			TotalStorageBytes: 107374182400,
			LastSeen:          time.Now(),
			Online:            true,
		},
	}

	for _, h := range hosts {
		if _, err := db.UpsertHost(h); err != nil {
			t.Fatalf("Failed to insert host: %v", err)
		}
	}

	if err := db.AddHostTag("prod-host-1", "production"); err != nil {
		t.Fatalf("Failed to add tag: %v", err)
	}

	if err := db.AddHostTag("prod-host-2", "production"); err != nil {
		t.Fatalf("Failed to add tag: %v", err)
	}

	if err := db.AddHostTag("dev-host-1", "development"); err != nil {
		t.Fatalf("Failed to add tag: %v", err)
	}

	prodHosts, err := db.GetHostsByTags([]string{"production"})
	if err != nil {
		t.Fatalf("Failed to get hosts by tag: %v", err)
	}

	if len(prodHosts) != 2 {
		t.Errorf("Expected 2 production hosts, got %d", len(prodHosts))
	}

	allHosts, err := db.GetHostsByTags([]string{})
	if err != nil {
		t.Fatalf("Failed to get all hosts: %v", err)
	}

	if len(allHosts) != 3 {
		t.Errorf("Expected 3 hosts when no tags specified, got %d", len(allHosts))
	}

	multiTagHosts, err := db.GetHostsByTags([]string{"production", "development"})
	if err != nil {
		t.Fatalf("Failed to get hosts by multiple tags: %v", err)
	}

	if len(multiTagHosts) != 3 {
		t.Errorf("Expected 3 hosts with OR tag logic, got %d", len(multiTagHosts))
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
