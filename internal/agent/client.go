package agent

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	pb "github.com/metorial/sentinel/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	collector *MetricsCollector
	executor  *ScriptExecutor
	conn      *grpc.ClientConn
	stream    pb.MetricsCollector_StreamMetricsClient
	hostname  string
}

func NewClient(collectorAddr string) (*Client, error) {
	collector, err := NewMetricsCollector()
	if err != nil {
		return nil, fmt.Errorf("create metrics collector: %w", err)
	}

	executor, err := NewScriptExecutor()
	if err != nil {
		return nil, fmt.Errorf("create script executor: %w", err)
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
		executor:  executor,
		conn:      conn,
		stream:    stream,
		hostname:  collector.hostname,
	}

	go c.receiveMessages()

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

	msg := &pb.AgentMessage{
		Payload: &pb.AgentMessage_Metrics{
			Metrics: metrics,
		},
	}

	if err := c.stream.Send(msg); err != nil {
		return fmt.Errorf("send to stream: %w", err)
	}

	return nil
}

func (c *Client) receiveMessages() {
	for {
		msg, err := c.stream.Recv()
		if err == io.EOF {
			log.Println("Stream closed by server")
			return
		}
		if err != nil {
			log.Printf("Error receiving message: %v", err)
			return
		}

		switch payload := msg.Payload.(type) {
		case *pb.CollectorMessage_Ack:
			ack := payload.Ack
			if !ack.Success {
				log.Printf("Server reported error: %s", ack.Message)
			}

		case *pb.CollectorMessage_ScriptCommand:
			cmd := payload.ScriptCommand
			log.Printf("Received script command: %s", cmd.ScriptId)
			go c.executeScript(cmd)

		default:
			log.Printf("Unknown message type: %T", payload)
		}
	}
}

func (c *Client) executeScript(cmd *pb.ScriptCommand) {
	alreadyRun, err := c.executor.HasExecuted(cmd.Sha256Hash)
	if err != nil {
		log.Printf("Error checking script execution history: %v", err)
		return
	}

	if alreadyRun {
		log.Printf("Script %s already executed, skipping", cmd.ScriptId)
		return
	}

	result, err := c.executor.Execute(cmd.ScriptId, cmd.Content, cmd.Sha256Hash)
	if err != nil {
		log.Printf("Error executing script %s: %v", cmd.ScriptId, err)
		return
	}

	msg := &pb.AgentMessage{
		Payload: &pb.AgentMessage_ScriptResult{
			ScriptResult: result,
		},
	}

	if err := c.stream.Send(msg); err != nil {
		log.Printf("Error sending script result: %v", err)
	} else {
		log.Printf("Script %s executed successfully with exit code %d", cmd.ScriptId, result.ExitCode)
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
