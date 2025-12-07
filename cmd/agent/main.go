package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/metorial/sentinel/internal/agent"
)

const (
	defaultConsulAddr     = "127.0.0.1:8500"
	defaultReportInterval = 10 * time.Second
	defaultRetryDelay     = 5 * time.Second
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	collectorURL := os.Getenv("COLLECTOR_URL")
	consulAddr := getEnv("CONSUL_HTTP_ADDR", "")

	if collectorURL == "" && consulAddr == "" {
		log.Fatal("Either COLLECTOR_URL or CONSUL_HTTP_ADDR must be set")
	}

	log.Printf("Starting agent service")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal: %v", sig)
		cancel()
	}()

	if collectorURL != "" {
		log.Printf("Using direct collector URL: %s", collectorURL)
		for {
			select {
			case <-ctx.Done():
				log.Println("Shutting down")
				return nil
			default:
				if err := runClient(ctx, collectorURL); err != nil {
					log.Printf("Client error: %v, retrying...", err)
					time.Sleep(defaultRetryDelay)
				}
			}
		}
	}

	if consulAddr == "" {
		consulAddr = defaultConsulAddr
	}
	log.Printf("Using Consul service discovery at: %s", consulAddr)

	// Retry service discovery creation indefinitely
	var discovery *agent.ServiceDiscovery
	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down")
			return nil
		default:
		}

		var err error
		discovery, err = agent.NewServiceDiscovery(consulAddr)
		if err != nil {
			log.Printf("Failed to create service discovery: %v, retrying...", err)
			time.Sleep(defaultRetryDelay)
			continue
		}
		break
	}

	addrChan := discovery.WatchCommander()

	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down")
			return nil

		case collectorAddr := <-addrChan:
			if err := runClient(ctx, collectorAddr); err != nil {
				log.Printf("Client error: %v, retrying...", err)
				time.Sleep(defaultRetryDelay)
			}
		}
	}
}

func runClient(ctx context.Context, collectorAddr string) error {
	log.Printf("Connecting to collector at: %s", collectorAddr)

	client, err := agent.NewClient(collectorAddr)
	if err != nil {
		return err
	}
	defer client.Close()

	log.Println("Connected successfully, streaming metrics")
	return client.Start(ctx, defaultReportInterval)
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
