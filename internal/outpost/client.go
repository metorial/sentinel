package outpost

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	pb "github.com/metorial/fleet/node-manager/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	collector *MetricsCollector
	conn      *grpc.ClientConn
	stream    pb.MetricsCollector_StreamMetricsClient
}

func NewClient(collectorAddr string) (*Client, error) {
	collector, err := NewMetricsCollector()
	if err != nil {
		return nil, fmt.Errorf("create metrics collector: %w", err)
	}

	conn, err := grpc.NewClient(collectorAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(10*1024*1024)),
	)
	if err != nil {
		return nil, fmt.Errorf("create grpc client: %w", err)
	}

	client := pb.NewMetricsCollectorClient(conn)
	stream, err := client.StreamMetrics(context.Background())
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create stream: %w", err)
	}

	c := &Client{
		collector: collector,
		conn:      conn,
		stream:    stream,
	}

	go c.receiveAcks()

	return c, nil
}

func (c *Client) Start(ctx context.Context, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := c.sendMetrics(); err != nil {
				return fmt.Errorf("send metrics: %w", err)
			}
		}
	}
}

func (c *Client) sendMetrics() error {
	metrics, err := c.collector.Collect()
	if err != nil {
		return fmt.Errorf("collect metrics: %w", err)
	}

	if err := c.stream.Send(metrics); err != nil {
		return fmt.Errorf("send to stream: %w", err)
	}

	return nil
}

func (c *Client) receiveAcks() {
	for {
		ack, err := c.stream.Recv()
		if err == io.EOF {
			log.Println("Stream closed by server")
			return
		}
		if err != nil {
			log.Printf("Error receiving ack: %v", err)
			return
		}

		if !ack.Success {
			log.Printf("Server reported error: %s", ack.Message)
		}
	}
}

func (c *Client) Close() error {
	if c.stream != nil {
		c.stream.CloseSend()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
