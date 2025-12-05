package collector

import (
	"context"
	"testing"
	"time"

	pb "github.com/metorial/fleet/node-manager/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"net"
)

const bufSize = 1024 * 1024

func TestStreamMetrics(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	server := NewServer(db)
	listener := bufconn.Listen(bufSize)

	grpcServer := grpc.NewServer()
	pb.RegisterMetricsCollectorServer(grpcServer, server)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewMetricsCollectorClient(conn)
	stream, err := client.StreamMetrics(ctx)
	if err != nil {
		t.Fatalf("Failed to create stream: %v", err)
	}

	metrics := &pb.HostMetrics{
		Hostname:  "test-host",
		Ip:        "192.168.1.100",
		Timestamp: time.Now().Unix(),
		Info: &pb.HostInfo{
			UptimeSeconds:     3600,
			CpuCores:          4,
			TotalMemoryBytes:  8589934592,
			TotalStorageBytes: 107374182400,
		},
		Usage: &pb.ResourceUsage{
			CpuPercent:       45.5,
			UsedMemoryBytes:  4294967296,
			UsedStorageBytes: 53687091200,
		},
	}

	msg := &pb.OutpostMessage{
		Payload: &pb.OutpostMessage_Metrics{
			Metrics: metrics,
		},
	}

	if err := stream.Send(msg); err != nil {
		t.Fatalf("Failed to send metrics: %v", err)
	}

	response, err := stream.Recv()
	if err != nil {
		t.Fatalf("Failed to receive response: %v", err)
	}

	ack := response.GetAck()
	if ack == nil {
		t.Fatal("Expected acknowledgment, got nil")
	}

	if !ack.Success {
		t.Errorf("Expected success, got: %s", ack.Message)
	}

	var count int
	err = db.conn.QueryRow("SELECT COUNT(*) FROM hosts WHERE hostname = ?", "test-host").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query host: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 host, got %d", count)
	}
}

func TestHandleMetricsMissingData(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	server := NewServer(db)

	tests := []struct {
		name    string
		metrics *pb.HostMetrics
		wantErr bool
	}{
		{
			name: "missing info",
			metrics: &pb.HostMetrics{
				Hostname:  "test-host",
				Ip:        "192.168.1.100",
				Timestamp: time.Now().Unix(),
				Usage: &pb.ResourceUsage{
					CpuPercent:       45.5,
					UsedMemoryBytes:  4294967296,
					UsedStorageBytes: 53687091200,
				},
			},
			wantErr: true,
		},
		{
			name: "missing usage",
			metrics: &pb.HostMetrics{
				Hostname:  "test-host",
				Ip:        "192.168.1.100",
				Timestamp: time.Now().Unix(),
				Info: &pb.HostInfo{
					UptimeSeconds:     3600,
					CpuCores:          4,
					TotalMemoryBytes:  8589934592,
					TotalStorageBytes: 107374182400,
				},
			},
			wantErr: true,
		},
		{
			name: "valid metrics",
			metrics: &pb.HostMetrics{
				Hostname:  "test-host",
				Ip:        "192.168.1.100",
				Timestamp: time.Now().Unix(),
				Info: &pb.HostInfo{
					UptimeSeconds:     3600,
					CpuCores:          4,
					TotalMemoryBytes:  8589934592,
					TotalStorageBytes: 107374182400,
				},
				Usage: &pb.ResourceUsage{
					CpuPercent:       45.5,
					UsedMemoryBytes:  4294967296,
					UsedStorageBytes: 53687091200,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := server.handleMetrics(tt.metrics)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleMetrics() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
