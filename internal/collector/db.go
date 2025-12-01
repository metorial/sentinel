package collector

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/metorial/fleet/node-manager/internal/models"
)

type DB struct {
	conn *sql.DB
}

func NewDB(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path)
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
