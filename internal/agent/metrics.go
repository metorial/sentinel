package agent

import (
	"fmt"
	"os"
	"time"

	pb "github.com/metorial/sentinel/proto"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

type MetricsCollector struct {
	hostname string
	ip       string
}

func NewMetricsCollector() (*MetricsCollector, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("get hostname: %w", err)
	}

	ip, err := getLocalIP()
	if err != nil {
		return nil, fmt.Errorf("get local ip: %w", err)
	}

	return &MetricsCollector{
		hostname: hostname,
		ip:       ip,
	}, nil
}

func (mc *MetricsCollector) Collect() (*pb.HostMetrics, error) {
	info, err := mc.collectInfo()
	if err != nil {
		return nil, fmt.Errorf("collect info: %w", err)
	}

	usage, err := mc.collectUsage()
	if err != nil {
		return nil, fmt.Errorf("collect usage: %w", err)
	}

	return &pb.HostMetrics{
		Hostname:  mc.hostname,
		Ip:        mc.ip,
		Timestamp: time.Now().Unix(),
		Info:      info,
		Usage:     usage,
	}, nil
}

func (mc *MetricsCollector) collectInfo() (*pb.HostInfo, error) {
	uptime, err := host.Uptime()
	if err != nil {
		return nil, fmt.Errorf("get uptime: %w", err)
	}

	cpuCores, err := cpu.Counts(true)
	if err != nil {
		return nil, fmt.Errorf("get cpu cores: %w", err)
	}

	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("get memory info: %w", err)
	}

	diskInfo, err := disk.Usage("/")
	if err != nil {
		return nil, fmt.Errorf("get disk info: %w", err)
	}

	return &pb.HostInfo{
		UptimeSeconds:     int64(uptime),
		CpuCores:          int32(cpuCores),
		TotalMemoryBytes:  int64(memInfo.Total),
		TotalStorageBytes: int64(diskInfo.Total),
	}, nil
}

func (mc *MetricsCollector) collectUsage() (*pb.ResourceUsage, error) {
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return nil, fmt.Errorf("get cpu percent: %w", err)
	}

	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("get memory usage: %w", err)
	}

	diskInfo, err := disk.Usage("/")
	if err != nil {
		return nil, fmt.Errorf("get disk usage: %w", err)
	}

	cpuPct := 0.0
	if len(cpuPercent) > 0 {
		cpuPct = cpuPercent[0]
	}

	return &pb.ResourceUsage{
		CpuPercent:       cpuPct,
		UsedMemoryBytes:  int64(memInfo.Used),
		UsedStorageBytes: int64(diskInfo.Used),
	}, nil
}

func getLocalIP() (string, error) {
	info, err := host.Info()
	if err != nil {
		return "", err
	}
	return info.HostID, nil
}
