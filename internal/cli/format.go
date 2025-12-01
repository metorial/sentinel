package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"
	"time"
)

func FormatJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func FormatHostsTable(data map[string]interface{}) error {
	hosts, ok := data["hosts"].([]interface{})
	if !ok {
		return fmt.Errorf("invalid hosts data")
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "HOSTNAME\tIP\tSTATUS\tCPU CORES\tMEMORY\tSTORAGE\tLAST SEEN")

	for _, h := range hosts {
		host := h.(map[string]interface{})

		status := "offline"
		if online, ok := host["online"].(bool); ok && online {
			status = "online"
		}

		cpuCores := formatNumber(host["cpu_cores"])
		memory := formatBytes(host["total_memory_bytes"])
		storage := formatBytes(host["total_storage_bytes"])
		lastSeen := formatTime(host["last_seen"])

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			getString(host["hostname"]),
			getString(host["ip"]),
			status,
			cpuCores,
			memory,
			storage,
			lastSeen,
		)
	}

	return w.Flush()
}

func FormatHostDetailTable(data map[string]interface{}) error {
	host, ok := data["host"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid host data")
	}

	fmt.Printf("Host: %s\n", getString(host["hostname"]))
	fmt.Printf("IP: %s\n", getString(host["ip"]))
	fmt.Printf("Status: %s\n", formatOnline(host["online"]))
	fmt.Printf("CPU Cores: %s\n", formatNumber(host["cpu_cores"]))
	fmt.Printf("Total Memory: %s\n", formatBytes(host["total_memory_bytes"]))
	fmt.Printf("Total Storage: %s\n", formatBytes(host["total_storage_bytes"]))
	fmt.Printf("Uptime: %s\n", formatUptime(host["uptime_seconds"]))
	fmt.Printf("Last Seen: %s\n", formatTime(host["last_seen"]))
	fmt.Printf("\n")

	usage, ok := data["usage"].([]interface{})
	if !ok || len(usage) == 0 {
		fmt.Println("No usage data available")
		return nil
	}

	fmt.Printf("Recent Usage (%d records):\n\n", len(usage))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TIMESTAMP\tCPU %\tMEMORY USED\tSTORAGE USED")

	for _, u := range usage {
		record := u.(map[string]interface{})
		fmt.Fprintf(w, "%s\t%s%%\t%s\t%s\n",
			formatTime(record["timestamp"]),
			formatFloat(record["cpu_percent"]),
			formatBytes(record["used_memory_bytes"]),
			formatBytes(record["used_storage_bytes"]),
		)
	}

	return w.Flush()
}

func FormatStatsTable(data map[string]interface{}) error {
	fmt.Println("Cluster Statistics:")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	fmt.Fprintf(w, "Total Hosts:\t%s\n", formatNumber(data["total_hosts"]))
	fmt.Fprintf(w, "Online Hosts:\t%s\n", formatNumber(data["online_hosts"]))
	fmt.Fprintf(w, "Offline Hosts:\t%s\n", formatNumber(data["offline_hosts"]))
	fmt.Fprintf(w, "Total CPU Cores:\t%s\n", formatNumber(data["total_cpu_cores"]))
	fmt.Fprintf(w, "Total Memory:\t%s\n", formatBytes(data["total_memory_bytes"]))
	fmt.Fprintf(w, "Total Storage:\t%s\n", formatBytes(data["total_storage_bytes"]))
	fmt.Fprintf(w, "Avg CPU Usage:\t%s%%\n", formatFloat(data["avg_cpu_percent"]))

	return w.Flush()
}

func getString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func formatNumber(v interface{}) string {
	switch n := v.(type) {
	case float64:
		return strconv.FormatInt(int64(n), 10)
	case int64:
		return strconv.FormatInt(n, 10)
	case int:
		return strconv.Itoa(n)
	default:
		return "0"
	}
}

func formatFloat(v interface{}) string {
	if f, ok := v.(float64); ok {
		return fmt.Sprintf("%.1f", f)
	}
	return "0.0"
}

func formatBytes(v interface{}) string {
	var bytes float64
	switch n := v.(type) {
	case float64:
		bytes = n
	case int64:
		bytes = float64(n)
	case int:
		bytes = float64(n)
	default:
		return "0 B"
	}

	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	for bytes >= 1024 && i < len(units)-1 {
		bytes /= 1024
		i++
	}

	return fmt.Sprintf("%.1f %s", bytes, units[i])
}

func formatTime(v interface{}) string {
	if s, ok := v.(string); ok {
		t, err := time.Parse(time.RFC3339, s)
		if err == nil {
			return t.Format("2006-01-02 15:04:05")
		}
		return s
	}
	return ""
}

func formatUptime(v interface{}) string {
	var seconds int64
	switch n := v.(type) {
	case float64:
		seconds = int64(n)
	case int64:
		seconds = n
	case int:
		seconds = int64(n)
	default:
		return "0s"
	}

	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func formatOnline(v interface{}) string {
	if online, ok := v.(bool); ok && online {
		return "online"
	}
	return "offline"
}
