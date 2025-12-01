package models

import "time"

type Host struct {
	ID               int64     `json:"id"`
	Hostname         string    `json:"hostname"`
	IP               string    `json:"ip"`
	UptimeSeconds    int64     `json:"uptime_seconds"`
	CPUCores         int32     `json:"cpu_cores"`
	TotalMemoryBytes int64     `json:"total_memory_bytes"`
	TotalStorageBytes int64    `json:"total_storage_bytes"`
	LastSeen         time.Time `json:"last_seen"`
	Online           bool      `json:"online"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type HostUsage struct {
	ID               int64     `json:"id"`
	HostID           int64     `json:"host_id"`
	Timestamp        time.Time `json:"timestamp"`
	CPUPercent       float64   `json:"cpu_percent"`
	UsedMemoryBytes  int64     `json:"used_memory_bytes"`
	UsedStorageBytes int64     `json:"used_storage_bytes"`
}
