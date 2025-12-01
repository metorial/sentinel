package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/metorial/fleet/node-manager/internal/outpost"
)

const (
	defaultConsulAddr      = "127.0.0.1:8500"
	defaultReportInterval  = 10 * time.Second
	defaultRetryDelay      = 5 * time.Second
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	consulAddr := getEnv("CONSUL_HTTP_ADDR", defaultConsulAddr)

	log.Printf("Starting outpost service")
	log.Printf("Consul address: %s", consulAddr)

	discovery, err := outpost.NewServiceDiscovery(consulAddr)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal: %v", sig)
		cancel()
	}()

	addrChan := discovery.WatchCollector()

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

	client, err := outpost.NewClient(collectorAddr)
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
