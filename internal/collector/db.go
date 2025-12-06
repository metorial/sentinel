package collector

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
	"github.com/metorial/fleet/node-manager/internal/models"
)

type DB struct {
	conn *sql.DB
}

func NewDB(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return db, nil
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS hosts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		hostname TEXT NOT NULL UNIQUE,
		ip TEXT NOT NULL,
		uptime_seconds INTEGER NOT NULL,
		cpu_cores INTEGER NOT NULL,
		total_memory_bytes INTEGER NOT NULL,
		total_storage_bytes INTEGER NOT NULL,
		last_seen TIMESTAMP NOT NULL,
		online BOOLEAN NOT NULL DEFAULT 1,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_hosts_hostname ON hosts(hostname);
	CREATE INDEX IF NOT EXISTS idx_hosts_last_seen ON hosts(last_seen);
	CREATE INDEX IF NOT EXISTS idx_hosts_online ON hosts(online);

	CREATE TABLE IF NOT EXISTS host_usage (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		host_id INTEGER NOT NULL,
		timestamp TIMESTAMP NOT NULL,
		cpu_percent REAL NOT NULL,
		used_memory_bytes INTEGER NOT NULL,
		used_storage_bytes INTEGER NOT NULL,
		FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_host_usage_host_id ON host_usage(host_id);
	CREATE INDEX IF NOT EXISTS idx_host_usage_timestamp ON host_usage(timestamp);

	CREATE TABLE IF NOT EXISTS tags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_tags_name ON tags(name);

	CREATE TABLE IF NOT EXISTS host_tags (
		host_id INTEGER NOT NULL,
		tag_id INTEGER NOT NULL,
		PRIMARY KEY (host_id, tag_id),
		FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE,
		FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS scripts (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		content TEXT NOT NULL,
		sha256_hash TEXT NOT NULL UNIQUE,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_scripts_sha256 ON scripts(sha256_hash);
	CREATE INDEX IF NOT EXISTS idx_scripts_name ON scripts(name);

	CREATE TABLE IF NOT EXISTS script_executions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		script_id TEXT NOT NULL,
		host_id INTEGER NOT NULL,
		sha256_hash TEXT NOT NULL,
		exit_code INTEGER NOT NULL,
		stdout TEXT,
		stderr TEXT,
		executed_at TIMESTAMP NOT NULL,
		FOREIGN KEY (script_id) REFERENCES scripts(id) ON DELETE CASCADE,
		FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_script_executions_script_id ON script_executions(script_id);
	CREATE INDEX IF NOT EXISTS idx_script_executions_host_id ON script_executions(host_id);
	CREATE INDEX IF NOT EXISTS idx_script_executions_hash_host ON script_executions(sha256_hash, host_id);
	`

	_, err := db.conn.Exec(schema)
	return err
}

func (db *DB) UpsertHost(host *models.Host) (int64, error) {
	query := `
	INSERT INTO hosts (hostname, ip, uptime_seconds, cpu_cores, total_memory_bytes, total_storage_bytes, last_seen, online, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(hostname) DO UPDATE SET
		ip = excluded.ip,
		uptime_seconds = excluded.uptime_seconds,
		cpu_cores = excluded.cpu_cores,
		total_memory_bytes = excluded.total_memory_bytes,
		total_storage_bytes = excluded.total_storage_bytes,
		last_seen = excluded.last_seen,
		online = excluded.online,
		updated_at = excluded.updated_at
	RETURNING id
	`

	var id int64
	err := db.conn.QueryRow(query, host.Hostname, host.IP, host.UptimeSeconds, host.CPUCores,
		host.TotalMemoryBytes, host.TotalStorageBytes, host.LastSeen, host.Online, time.Now()).Scan(&id)
	return id, err
}

func (db *DB) InsertUsage(usage *models.HostUsage) error {
	query := `INSERT INTO host_usage (host_id, timestamp, cpu_percent, used_memory_bytes, used_storage_bytes)
	          VALUES (?, ?, ?, ?, ?)`
	_, err := db.conn.Exec(query, usage.HostID, usage.Timestamp, usage.CPUPercent,
		usage.UsedMemoryBytes, usage.UsedStorageBytes)
	return err
}

func (db *DB) MarkInactive(threshold time.Duration) error {
	query := `UPDATE hosts SET online = 0 WHERE last_seen < ? AND online = 1`
	_, err := db.conn.Exec(query, time.Now().Add(-threshold))
	return err
}

func (db *DB) CleanupOldUsage(retention time.Duration) error {
	query := `DELETE FROM host_usage WHERE timestamp < ?`
	_, err := db.conn.Exec(query, time.Now().Add(-retention))
	return err
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) QueryRow(query string, args ...interface{}) *sql.Row {
	return db.conn.QueryRow(query, args...)
}

func (db *DB) GetAllHosts() ([]models.Host, error) {
	query := `SELECT id, hostname, ip, uptime_seconds, cpu_cores, total_memory_bytes,
	          total_storage_bytes, last_seen, online, created_at, updated_at
	          FROM hosts ORDER BY hostname`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hosts []models.Host
	for rows.Next() {
		var h models.Host
		err := rows.Scan(&h.ID, &h.Hostname, &h.IP, &h.UptimeSeconds, &h.CPUCores,
			&h.TotalMemoryBytes, &h.TotalStorageBytes, &h.LastSeen, &h.Online,
			&h.CreatedAt, &h.UpdatedAt)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}

	return hosts, rows.Err()
}

func (db *DB) GetHost(hostname string) (*models.Host, error) {
	query := `SELECT id, hostname, ip, uptime_seconds, cpu_cores, total_memory_bytes,
	          total_storage_bytes, last_seen, online, created_at, updated_at
	          FROM hosts WHERE hostname = ?`

	var h models.Host
	err := db.conn.QueryRow(query, hostname).Scan(&h.ID, &h.Hostname, &h.IP,
		&h.UptimeSeconds, &h.CPUCores, &h.TotalMemoryBytes, &h.TotalStorageBytes,
		&h.LastSeen, &h.Online, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &h, nil
}

func (db *DB) GetHostUsage(hostname string, limit int) ([]models.HostUsage, error) {
	query := `SELECT hu.id, hu.host_id, hu.timestamp, hu.cpu_percent,
	          hu.used_memory_bytes, hu.used_storage_bytes
	          FROM host_usage hu
	          JOIN hosts h ON hu.host_id = h.id
	          WHERE h.hostname = ?
	          ORDER BY hu.timestamp DESC
	          LIMIT ?`

	rows, err := db.conn.Query(query, hostname, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usage []models.HostUsage
	for rows.Next() {
		var u models.HostUsage
		err := rows.Scan(&u.ID, &u.HostID, &u.Timestamp, &u.CPUPercent,
			&u.UsedMemoryBytes, &u.UsedStorageBytes)
		if err != nil {
			return nil, err
		}
		usage = append(usage, u)
	}

	return usage, rows.Err()
}

func (db *DB) GetClusterStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	var totalHosts, onlineHosts int
	err := db.conn.QueryRow("SELECT COUNT(*), SUM(CASE WHEN online = 1 THEN 1 ELSE 0 END) FROM hosts").
		Scan(&totalHosts, &onlineHosts)
	if err != nil {
		return nil, err
	}

	stats["total_hosts"] = totalHosts
	stats["online_hosts"] = onlineHosts
	stats["offline_hosts"] = totalHosts - onlineHosts

	var totalCPUCores, totalMemoryBytes, totalStorageBytes int64
	err = db.conn.QueryRow(`SELECT SUM(cpu_cores), SUM(total_memory_bytes), SUM(total_storage_bytes)
	                        FROM hosts WHERE online = 1`).
		Scan(&totalCPUCores, &totalMemoryBytes, &totalStorageBytes)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	stats["total_cpu_cores"] = totalCPUCores
	stats["total_memory_bytes"] = totalMemoryBytes
	stats["total_storage_bytes"] = totalStorageBytes

	var avgCPUPercent sql.NullFloat64
	err = db.conn.QueryRow(`SELECT AVG(cpu_percent) FROM host_usage
	                        WHERE timestamp > datetime('now', '-5 minutes')`).
		Scan(&avgCPUPercent)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	if avgCPUPercent.Valid {
		stats["avg_cpu_percent"] = avgCPUPercent.Float64
	} else {
		stats["avg_cpu_percent"] = 0.0
	}

	return stats, nil
}

// CreateScript creates a new script
func (db *DB) CreateScript(script *models.Script) error {
	query := `INSERT INTO scripts (id, name, content, sha256_hash, created_at)
	          VALUES (?, ?, ?, ?, ?)`
	_, err := db.conn.Exec(query, script.ID, script.Name, script.Content, script.SHA256Hash, script.CreatedAt)
	return err
}

// GetScript retrieves a script by ID
func (db *DB) GetScript(id string) (*models.Script, error) {
	query := `SELECT id, name, content, sha256_hash, created_at FROM scripts WHERE id = ?`
	var s models.Script
	err := db.conn.QueryRow(query, id).Scan(&s.ID, &s.Name, &s.Content, &s.SHA256Hash, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// GetAllScripts retrieves all scripts
func (db *DB) GetAllScripts() ([]models.Script, error) {
	query := `SELECT id, name, content, sha256_hash, created_at FROM scripts ORDER BY created_at DESC`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scripts []models.Script
	for rows.Next() {
		var s models.Script
		if err := rows.Scan(&s.ID, &s.Name, &s.Content, &s.SHA256Hash, &s.CreatedAt); err != nil {
			return nil, err
		}
		scripts = append(scripts, s)
	}
	return scripts, rows.Err()
}

// DeleteScript deletes a script by ID
func (db *DB) DeleteScript(id string) error {
	query := `DELETE FROM scripts WHERE id = ?`
	_, err := db.conn.Exec(query, id)
	return err
}

// RecordScriptExecution records the result of a script execution
func (db *DB) RecordScriptExecution(exec *models.ScriptExecution) error {
	query := `INSERT INTO script_executions (script_id, host_id, sha256_hash, exit_code, stdout, stderr, executed_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := db.conn.Exec(query, exec.ScriptID, exec.HostID, exec.SHA256Hash, exec.ExitCode, exec.Stdout, exec.Stderr, exec.ExecutedAt)
	return err
}

// GetScriptExecutions retrieves executions for a script
func (db *DB) GetScriptExecutions(scriptID string) ([]models.ScriptExecution, error) {
	query := `SELECT se.id, se.script_id, se.host_id, h.hostname, se.sha256_hash, se.exit_code, se.stdout, se.stderr, se.executed_at
	          FROM script_executions se
	          JOIN hosts h ON se.host_id = h.id
	          WHERE se.script_id = ?
	          ORDER BY se.executed_at DESC`
	rows, err := db.conn.Query(query, scriptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var execs []models.ScriptExecution
	for rows.Next() {
		var e models.ScriptExecution
		if err := rows.Scan(&e.ID, &e.ScriptID, &e.HostID, &e.Hostname, &e.SHA256Hash, &e.ExitCode, &e.Stdout, &e.Stderr, &e.ExecutedAt); err != nil {
			return nil, err
		}
		execs = append(execs, e)
	}
	return execs, rows.Err()
}

// HasScriptExecuted checks if a script with given hash has been executed on a host
func (db *DB) HasScriptExecuted(hostname, sha256Hash string) (bool, error) {
	query := `SELECT COUNT(*) FROM script_executions se
	          JOIN hosts h ON se.host_id = h.id
	          WHERE h.hostname = ? AND se.sha256_hash = ?`
	var count int
	err := db.conn.QueryRow(query, hostname, sha256Hash).Scan(&count)
	return count > 0, err
}

// CreateTag creates a new tag
func (db *DB) CreateTag(name string) (int64, error) {
	query := `INSERT INTO tags (name) VALUES (?) RETURNING id`
	var id int64
	err := db.conn.QueryRow(query, name).Scan(&id)
	return id, err
}

// GetOrCreateTag gets a tag by name or creates it if it doesn't exist
func (db *DB) GetOrCreateTag(name string) (int64, error) {
	query := `SELECT id FROM tags WHERE name = ?`
	var id int64
	err := db.conn.QueryRow(query, name).Scan(&id)
	if err == sql.ErrNoRows {
		return db.CreateTag(name)
	}
	return id, err
}

// GetAllTags retrieves all tags
func (db *DB) GetAllTags() ([]models.Tag, error) {
	query := `SELECT id, name, created_at FROM tags ORDER BY name`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []models.Tag
	for rows.Next() {
		var t models.Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// AddHostTag adds a tag to a host
func (db *DB) AddHostTag(hostname, tagName string) error {
	tagID, err := db.GetOrCreateTag(tagName)
	if err != nil {
		return err
	}

	host, err := db.GetHost(hostname)
	if err != nil {
		return err
	}

	query := `INSERT OR IGNORE INTO host_tags (host_id, tag_id) VALUES (?, ?)`
	_, err = db.conn.Exec(query, host.ID, tagID)
	return err
}

// RemoveHostTag removes a tag from a host
func (db *DB) RemoveHostTag(hostname, tagName string) error {
	query := `DELETE FROM host_tags WHERE host_id = (SELECT id FROM hosts WHERE hostname = ?)
	          AND tag_id = (SELECT id FROM tags WHERE name = ?)`
	_, err := db.conn.Exec(query, hostname, tagName)
	return err
}

// GetHostTags retrieves all tags for a host
func (db *DB) GetHostTags(hostname string) ([]string, error) {
	query := `SELECT t.name FROM tags t
	          JOIN host_tags ht ON t.id = ht.tag_id
	          JOIN hosts h ON ht.host_id = h.id
	          WHERE h.hostname = ?
	          ORDER BY t.name`
	rows, err := db.conn.Query(query, hostname)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

// GetHostsByTags retrieves hosts that have ANY of the specified tags (OR logic)
func (db *DB) GetHostsByTags(tags []string) ([]models.Host, error) {
	if len(tags) == 0 {
		return db.GetAllHosts()
	}

	placeholders := ""
	args := make([]interface{}, len(tags))
	for i, tag := range tags {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args[i] = tag
	}

	query := `SELECT DISTINCT h.id, h.hostname, h.ip, h.uptime_seconds, h.cpu_cores, h.total_memory_bytes,
	          h.total_storage_bytes, h.last_seen, h.online, h.created_at, h.updated_at
	          FROM hosts h
	          JOIN host_tags ht ON h.id = ht.host_id
	          JOIN tags t ON ht.tag_id = t.id
	          WHERE t.name IN (` + placeholders + `)
	          ORDER BY h.hostname`

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hosts []models.Host
	for rows.Next() {
		var h models.Host
		err := rows.Scan(&h.ID, &h.Hostname, &h.IP, &h.UptimeSeconds, &h.CPUCores,
			&h.TotalMemoryBytes, &h.TotalStorageBytes, &h.LastSeen, &h.Online,
			&h.CreatedAt, &h.UpdatedAt)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}
