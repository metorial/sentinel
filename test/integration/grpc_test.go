package integration

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/metorial/fleet/node-manager/internal/collector"
	"github.com/metorial/fleet/node-manager/internal/outpost"
	pb "github.com/metorial/fleet/node-manager/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestFullGRPCFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	dbPath := t.TempDir() + "/test.db"
	defer os.Remove(dbPath)

	db, err := collector.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	grpcServer := grpc.NewServer()
	pb.RegisterMetricsCollectorServer(grpcServer, collector.NewServer(db))

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	serverAddr := listener.Addr().String()
	t.Logf("Server listening on %s", serverAddr)

	mc, err := outpost.NewMetricsCollector()
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}

	conn, err := grpc.NewClient(serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer conn.Close()

	client := pb.NewMetricsCollectorClient(conn)
	stream, err := client.StreamMetrics(context.Background())
	if err != nil {
		t.Fatalf("Failed to create stream: %v", err)
	}

	metrics, err := mc.Collect()
	if err != nil {
		t.Fatalf("Failed to collect metrics: %v", err)
	}

	for i := 0; i < 5; i++ {
		if err := stream.Send(metrics); err != nil {
			t.Fatalf("Failed to send metrics: %v", err)
		}

		ack, err := stream.Recv()
		if err != nil {
			t.Fatalf("Failed to receive ack: %v", err)
		}

		if !ack.Success {
			t.Errorf("Expected success, got: %s", ack.Message)
		}

		time.Sleep(100 * time.Millisecond)
		metrics.Timestamp = time.Now().Unix()
	}

	stream.CloseSend()

	var hostCount int
	err = db.QueryRow("SELECT COUNT(*) FROM hosts WHERE hostname = ?", metrics.Hostname).Scan(&hostCount)
	if err != nil {
		t.Fatalf("Failed to query host count: %v", err)
	}

	if hostCount != 1 {
		t.Errorf("Expected 1 host, got %d", hostCount)
	}

	var usageCount int
	err = db.QueryRow("SELECT COUNT(*) FROM host_usage").Scan(&usageCount)
	if err != nil {
		t.Fatalf("Failed to query usage count: %v", err)
	}

	if usageCount != 5 {
		t.Errorf("Expected 5 usage records, got %d", usageCount)
	}
}

func TestMultipleClients(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	dbPath := t.TempDir() + "/test.db"
	defer os.Remove(dbPath)

	db, err := collector.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	grpcServer := grpc.NewServer()
	pb.RegisterMetricsCollectorServer(grpcServer, collector.NewServer(db))

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	serverAddr := listener.Addr().String()

	numClients := 3
	done := make(chan bool, numClients)

	for i := 0; i < numClients; i++ {
		go func(clientID int) {
			conn, err := grpc.NewClient(serverAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				t.Errorf("Client %d: Failed to create connection: %v", clientID, err)
				done <- false
				return
			}
			defer conn.Close()

			client := pb.NewMetricsCollectorClient(conn)
			stream, err := client.StreamMetrics(context.Background())
			if err != nil {
				t.Errorf("Client %d: Failed to create stream: %v", clientID, err)
				done <- false
				return
			}

			metrics := &pb.HostMetrics{
				Hostname:  fmt.Sprintf("host-%d", clientID),
				Ip:        fmt.Sprintf("192.168.1.%d", clientID+100),
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

			for j := 0; j < 3; j++ {
				if err := stream.Send(metrics); err != nil {
					t.Errorf("Client %d: Failed to send metrics: %v", clientID, err)
					done <- false
					return
				}

				ack, err := stream.Recv()
				if err != nil {
					t.Errorf("Client %d: Failed to receive ack: %v", clientID, err)
					done <- false
					return
				}

				if !ack.Success {
					t.Errorf("Client %d: Expected success, got: %s", clientID, ack.Message)
				}

				time.Sleep(50 * time.Millisecond)
			}

			stream.CloseSend()
			done <- true
		}(i)
	}

	for i := 0; i < numClients; i++ {
		select {
		case success := <-done:
			if !success {
				t.Error("Client failed")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for clients")
		}
	}

	time.Sleep(200 * time.Millisecond)

	var hostCount int
	err = db.QueryRow("SELECT COUNT(*) FROM hosts").Scan(&hostCount)
	if err != nil {
		t.Fatalf("Failed to query host count: %v", err)
	}

	if hostCount != numClients {
		t.Errorf("Expected %d hosts, got %d", numClients, hostCount)
	}

	var usageCount int
	err = db.QueryRow("SELECT COUNT(*) FROM host_usage").Scan(&usageCount)
	if err != nil {
		t.Fatalf("Failed to query usage count: %v", err)
	}

	expected := numClients * 3
	if usageCount != expected {
		t.Errorf("Expected %d usage records, got %d", expected, usageCount)
	}
}

func TestInactiveHostMarking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	dbPath := t.TempDir() + "/test.db"
	defer os.Remove(dbPath)

	db, err := collector.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	grpcServer := grpc.NewServer()
	pb.RegisterMetricsCollectorServer(grpcServer, collector.NewServer(db))

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	serverAddr := listener.Addr().String()

	conn, err := grpc.NewClient(serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer conn.Close()

	client := pb.NewMetricsCollectorClient(conn)
	stream, err := client.StreamMetrics(context.Background())
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

	if err := stream.Send(metrics); err != nil {
		t.Fatalf("Failed to send metrics: %v", err)
	}

	if _, err := stream.Recv(); err != nil {
		t.Fatalf("Failed to receive ack: %v", err)
	}

	var online bool
	err = db.QueryRow("SELECT online FROM hosts WHERE hostname = ?", "test-host").Scan(&online)
	if err != nil {
		t.Fatalf("Failed to query host: %v", err)
	}

	if !online {
		t.Error("Expected host to be online")
	}

	if err := db.MarkInactive(1 * time.Second); err != nil {
		t.Fatalf("Failed to mark inactive: %v", err)
	}

	time.Sleep(1100 * time.Millisecond)

	if err := db.MarkInactive(1 * time.Second); err != nil {
		t.Fatalf("Failed to mark inactive: %v", err)
	}

	err = db.QueryRow("SELECT online FROM hosts WHERE hostname = ?", "test-host").Scan(&online)
	if err != nil {
		t.Fatalf("Failed to query host: %v", err)
	}

	if online {
		t.Error("Expected host to be offline after timeout")
	}

	stream.CloseSend()
}
