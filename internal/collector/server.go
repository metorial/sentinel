package collector

import (
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/metorial/fleet/node-manager/internal/models"
	pb "github.com/metorial/fleet/node-manager/proto"
)

type Server struct {
	pb.UnimplementedMetricsCollectorServer
	db      *DB
	streams map[string]pb.MetricsCollector_StreamMetricsServer
	mu      sync.RWMutex
}

func NewServer(db *DB) *Server {
	return &Server{
		db:      db,
		streams: make(map[string]pb.MetricsCollector_StreamMetricsServer),
	}
}

func (s *Server) StreamMetrics(stream pb.MetricsCollector_StreamMetricsServer) error {
	ctx := stream.Context()
	log.Println("New client connected")

	var hostname string

	defer func() {
		if hostname != "" {
			s.mu.Lock()
			delete(s.streams, hostname)
			s.mu.Unlock()
			log.Printf("Removed stream for host: %s", hostname)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			log.Println("Client disconnected:", ctx.Err())
			return ctx.Err()
		default:
		}

		msg, err := stream.Recv()
		if err == io.EOF {
			log.Println("Client closed stream")
			return nil
		}
		if err != nil {
			log.Printf("Error receiving message: %v", err)
			return err
		}

		switch payload := msg.Payload.(type) {
		case *pb.OutpostMessage_Metrics:
			metrics := payload.Metrics
			if hostname == "" {
				hostname = metrics.Hostname
				s.mu.Lock()
				s.streams[hostname] = stream
				s.mu.Unlock()
				log.Printf("Registered stream for host: %s", hostname)
			}

			if err := s.handleMetrics(metrics); err != nil {
				log.Printf("Error handling metrics from %s: %v", metrics.Hostname, err)
				if err := stream.Send(&pb.CollectorMessage{
					Payload: &pb.CollectorMessage_Ack{
						Ack: &pb.Acknowledgment{
							Success: false,
							Message: err.Error(),
						},
					},
				}); err != nil {
					return err
				}
				continue
			}

			if err := stream.Send(&pb.CollectorMessage{
				Payload: &pb.CollectorMessage_Ack{
					Ack: &pb.Acknowledgment{
						Success: true,
						Message: "received",
					},
				},
			}); err != nil {
				return err
			}

		case *pb.OutpostMessage_ScriptResult:
			result := payload.ScriptResult
			if err := s.handleScriptResult(hostname, result); err != nil {
				log.Printf("Error handling script result: %v", err)
			}

		default:
			log.Printf("Unknown message type: %T", payload)
		}
	}
}

func (s *Server) handleMetrics(metrics *pb.HostMetrics) error {
	if metrics.Info == nil || metrics.Usage == nil {
		return fmt.Errorf("missing info or usage data")
	}

	host := &models.Host{
		Hostname:          metrics.Hostname,
		IP:                metrics.Ip,
		UptimeSeconds:     metrics.Info.UptimeSeconds,
		CPUCores:          metrics.Info.CpuCores,
		TotalMemoryBytes:  metrics.Info.TotalMemoryBytes,
		TotalStorageBytes: metrics.Info.TotalStorageBytes,
		LastSeen:          time.Unix(metrics.Timestamp, 0),
		Online:            true,
	}

	hostID, err := s.db.UpsertHost(host)
	if err != nil {
		return fmt.Errorf("upsert host: %w", err)
	}

	usage := &models.HostUsage{
		HostID:           hostID,
		Timestamp:        time.Unix(metrics.Timestamp, 0),
		CPUPercent:       metrics.Usage.CpuPercent,
		UsedMemoryBytes:  metrics.Usage.UsedMemoryBytes,
		UsedStorageBytes: metrics.Usage.UsedStorageBytes,
	}

	if err := s.db.InsertUsage(usage); err != nil {
		return fmt.Errorf("insert usage: %w", err)
	}

	return nil
}

func (s *Server) handleScriptResult(hostname string, result *pb.ScriptResult) error {
	host, err := s.db.GetHost(hostname)
	if err != nil {
		return fmt.Errorf("get host: %w", err)
	}

	exec := &models.ScriptExecution{
		ScriptID:   result.ScriptId,
		HostID:     host.ID,
		SHA256Hash: result.Sha256Hash,
		ExitCode:   result.ExitCode,
		Stdout:     result.Stdout,
		Stderr:     result.Stderr,
		ExecutedAt: time.Unix(result.ExecutedAt, 0),
	}

	if err := s.db.RecordScriptExecution(exec); err != nil {
		return fmt.Errorf("record execution: %w", err)
	}

	log.Printf("Script %s executed on %s with exit code %d", result.ScriptId, hostname, result.ExitCode)
	return nil
}

// DistributeScript sends a script to specified hosts
func (s *Server) DistributeScript(script *models.Script, hosts []models.Host) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cmd := &pb.ScriptCommand{
		ScriptId:   script.ID,
		Content:    script.Content,
		Sha256Hash: script.SHA256Hash,
	}

	for _, host := range hosts {
		if !host.Online {
			log.Printf("Skipping offline host: %s", host.Hostname)
			continue
		}

		stream, ok := s.streams[host.Hostname]
		if !ok {
			log.Printf("No active stream for host: %s", host.Hostname)
			continue
		}

		if err := stream.Send(&pb.CollectorMessage{
			Payload: &pb.CollectorMessage_ScriptCommand{
				ScriptCommand: cmd,
			},
		}); err != nil {
			log.Printf("Failed to send script to %s: %v", host.Hostname, err)
		} else {
			log.Printf("Script %s sent to %s", script.ID, host.Hostname)
		}
	}
}
