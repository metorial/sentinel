package agent

import (
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	pb "github.com/metorial/sentinel/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

type mockServer struct {
	pb.UnimplementedMetricsCollectorServer
	receivedMetrics []*pb.HostMetrics
	mu              sync.Mutex
}

func (m *mockServer) StreamMetrics(stream pb.MetricsCollector_StreamMetricsServer) error {
	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}

		if metrics := msg.GetMetrics(); metrics != nil {
			m.mu.Lock()
			m.receivedMetrics = append(m.receivedMetrics, metrics)
			m.mu.Unlock()
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
	}
}

func (m *mockServer) getReceivedMetrics() []*pb.HostMetrics {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]*pb.HostMetrics{}, m.receivedMetrics...)
}

func setupMockServer(t *testing.T) (*mockServer, *bufconn.Listener, func()) {
	t.Helper()

	mock := &mockServer{
		receivedMetrics: make([]*pb.HostMetrics, 0),
	}

	listener := bufconn.Listen(bufSize)
	server := grpc.NewServer()
	pb.RegisterMetricsCollectorServer(server, mock)

	go func() {
		if err := server.Serve(listener); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	cleanup := func() {
		server.Stop()
		listener.Close()
	}

	return mock, listener, cleanup
}

func TestNewClient(t *testing.T) {
	mock, listener, cleanup := setupMockServer(t)
	defer cleanup()

	_ = mock

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

	collector, err := NewMetricsCollector()
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}

	grpcClient := pb.NewMetricsCollectorClient(conn)
	stream, err := grpcClient.StreamMetrics(ctx)
	if err != nil {
		t.Fatalf("Failed to create stream: %v", err)
	}

	client := &Client{
		collector: collector,
		conn:      conn,
		stream:    stream,
	}

	if client.collector == nil {
		t.Error("Expected non-nil collector")
	}

	if client.stream == nil {
		t.Error("Expected non-nil stream")
	}
}

func TestClientSendMetrics(t *testing.T) {
	mock, listener, cleanup := setupMockServer(t)
	defer cleanup()

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

	collector, err := NewMetricsCollector()
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}

	grpcClient := pb.NewMetricsCollectorClient(conn)
	stream, err := grpcClient.StreamMetrics(ctx)
	if err != nil {
		t.Fatalf("Failed to create stream: %v", err)
	}

	executor, err := NewScriptExecutor()
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}

	client := &Client{
		collector: collector,
		executor:  executor,
		conn:      conn,
		stream:    stream,
		hostname:  collector.hostname,
	}
	go client.receiveMessages()

	if err := client.sendMetrics(); err != nil {
		// Skip test if CPU metrics not available (CGO disabled or platform limitation)
		if strings.Contains(err.Error(), "not implemented yet") {
			t.Skip("Skipping test: CPU metrics not available without CGO")
		}
		t.Fatalf("Failed to send metrics: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	received := mock.getReceivedMetrics()
	if len(received) != 1 {
		t.Errorf("Expected 1 metric, got %d", len(received))
	}

	if len(received) > 0 {
		if received[0].Hostname == "" {
			t.Error("Expected non-empty hostname")
		}
	}
}

func TestClientStart(t *testing.T) {
	mock, listener, cleanup := setupMockServer(t)
	defer cleanup()

	clientCtx := context.Background()

	conn, err := grpc.DialContext(clientCtx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	collector, err := NewMetricsCollector()
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}

	grpcClient := pb.NewMetricsCollectorClient(conn)
	stream, err := grpcClient.StreamMetrics(clientCtx)
	if err != nil {
		t.Fatalf("Failed to create stream: %v", err)
	}

	executor, err := NewScriptExecutor()
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}

	client := &Client{
		collector: collector,
		executor:  executor,
		conn:      conn,
		stream:    stream,
		hostname:  collector.hostname,
	}
	go client.receiveMessages()

	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()

	err = client.Start(ctx, 100*time.Millisecond)
	if err != context.DeadlineExceeded {
		// Skip test if CPU metrics not available (CGO disabled or platform limitation)
		if err != nil && strings.Contains(err.Error(), "not implemented yet") {
			t.Skip("Skipping test: CPU metrics not available without CGO")
		}
		t.Logf("Got error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	received := mock.getReceivedMetrics()
	if len(received) < 1 {
		// If we got the not implemented error, this is expected
		if err != nil && strings.Contains(err.Error(), "not implemented yet") {
			t.Skip("Skipping test: CPU metrics not available without CGO")
		}
		t.Errorf("Expected at least 1 metric, got %d", len(received))
	}

	t.Logf("Received %d metrics", len(received))
}
