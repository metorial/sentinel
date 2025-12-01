package collector

import (
	"fmt"
	"io"
	"log"
	"time"

	"github.com/metorial/fleet/node-manager/internal/models"
	pb "github.com/metorial/fleet/node-manager/proto"
)

type Server struct {
	pb.UnimplementedMetricsCollectorServer
	db *DB
}

func NewServer(db *DB) *Server {
	return &Server{db: db}
}

func (s *Server) StreamMetrics(stream pb.MetricsCollector_StreamMetricsServer) error {
	ctx := stream.Context()
	log.Println("New client connected")

	for {
		select {
		case <-ctx.Done():
			log.Println("Client disconnected:", ctx.Err())
			return ctx.Err()
		default:
		}

		metrics, err := stream.Recv()
		if err == io.EOF {
			log.Println("Client closed stream")
			return nil
		}
		if err != nil {
			log.Printf("Error receiving metrics: %v", err)
			return err
		}

		if err := s.handleMetrics(metrics); err != nil {
			log.Printf("Error handling metrics from %s: %v", metrics.Hostname, err)
			if err := stream.Send(&pb.Acknowledgment{
				Success: false,
				Message: err.Error(),
			}); err != nil {
				return err
			}
			continue
		}

		if err := stream.Send(&pb.Acknowledgment{
			Success: true,
			Message: "received",
		}); err != nil {
			return err
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
