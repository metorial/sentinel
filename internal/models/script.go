package models

import "time"

type Script struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Content    string    `json:"content"`
	SHA256Hash string    `json:"sha256_hash"`
	CreatedAt  time.Time `json:"created_at"`
}

type ScriptExecution struct {
	ID          int64     `json:"id"`
	ScriptID    string    `json:"script_id"`
	HostID      int64     `json:"host_id"`
	Hostname    string    `json:"hostname,omitempty"`
	SHA256Hash  string    `json:"sha256_hash"`
	ExitCode    int32     `json:"exit_code"`
	Stdout      string    `json:"stdout"`
	Stderr      string    `json:"stderr"`
	ExecutedAt  time.Time `json:"executed_at"`
}

type Tag struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}
