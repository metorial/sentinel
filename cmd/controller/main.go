package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	consul "github.com/hashicorp/consul/api"
	"github.com/metorial/sentinel/internal/commander"
	pb "github.com/metorial/sentinel/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const (
	defaultPort            = "9090"
	defaultHTTPPort        = "8080"
	defaultDBPath          = "/data/metrics.db"
	defaultInactiveTimeout = 60 * time.Second
	defaultCleanupInterval = 5 * time.Minute
	defaultRetentionPeriod = 7 * 24 * time.Hour
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	port := getEnv("PORT", defaultPort)
	httpPort := getEnv("HTTP_PORT", defaultHTTPPort)
	dbPath := getEnv("DB_PATH", defaultDBPath)

	db, err := commander.NewDB(dbPath)
	if err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}
	defer db.Close()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	grpcServer := grpc.NewServer()
	server := commander.NewServer(db)
	pb.RegisterMetricsCollectorServer(grpcServer, server)

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	mux := http.NewServeMux()
	api := commander.NewAPI(db, server)
	api.RegisterRoutes(mux)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", httpPort),
		Handler: mux,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startMaintenanceTasks(ctx, db)

	if err := registerConsul(port, httpPort); err != nil {
		log.Printf("Warning: failed to register with Consul: %v", err)
	}
	defer deregisterConsul()

	errChan := make(chan error, 2)
	go func() {
		log.Printf("gRPC server listening on :%s", port)
		errChan <- grpcServer.Serve(lis)
	}()

	go func() {
		log.Printf("HTTP API server listening on :%s", httpPort)
		errChan <- httpServer.ListenAndServe()
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		return err
	case sig := <-sigChan:
		log.Printf("Received signal: %v", sig)
		grpcServer.GracefulStop()
		httpServer.Shutdown(context.Background())
		return nil
	}
}

func startMaintenanceTasks(ctx context.Context, db *commander.DB) {
	inactiveTicker := time.NewTicker(10 * time.Second)
	cleanupTicker := time.NewTicker(defaultCleanupInterval)
	defer inactiveTicker.Stop()
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-inactiveTicker.C:
			if err := db.MarkInactive(defaultInactiveTimeout); err != nil {
				log.Printf("Error marking inactive hosts: %v", err)
			}
		case <-cleanupTicker.C:
			if err := db.CleanupOldUsage(defaultRetentionPeriod); err != nil {
				log.Printf("Error cleaning up old usage data: %v", err)
			}
		}
	}
}

func registerConsul(port, httpPort string) error {
	consulAddr := getEnv("CONSUL_HTTP_ADDR", "")
	if consulAddr == "" {
		return nil
	}

	config := consul.DefaultConfig()
	config.Address = consulAddr
	client, err := consul.NewClient(config)
	if err != nil {
		return err
	}

	nodeIP := getEnv("NOMAD_IP_grpc", "")
	if nodeIP == "" {
		nodeIP = getLocalIP()
	}

	registration := &consul.AgentServiceRegistration{
		ID:      "sentinel-controller",
		Name:    "sentinel-controller",
		Port:    mustAtoi(port),
		Address: nodeIP,
		Check: &consul.AgentServiceCheck{
			GRPC:                           fmt.Sprintf("%s:%s", nodeIP, port),
			Interval:                       "10s",
			Timeout:                        "5s",
			DeregisterCriticalServiceAfter: "30s",
		},
		Tags: []string{"metrics", "commander", "grpc"},
	}

	if err := client.Agent().ServiceRegister(registration); err != nil {
		return err
	}

	httpRegistration := &consul.AgentServiceRegistration{
		ID:      "sentinel-controller-http",
		Name:    "sentinel-controller-http",
		Port:    mustAtoi(httpPort),
		Address: nodeIP,
		Check: &consul.AgentServiceCheck{
			HTTP:                           fmt.Sprintf("http://%s:%s/api/v1/health", nodeIP, httpPort),
			Interval:                       "10s",
			Timeout:                        "5s",
			DeregisterCriticalServiceAfter: "30s",
		},
		Tags: []string{"metrics", "commander", "http", "api"},
	}

	return client.Agent().ServiceRegister(httpRegistration)
}

func deregisterConsul() {
	consulAddr := getEnv("CONSUL_HTTP_ADDR", "")
	if consulAddr == "" {
		return
	}

	config := consul.DefaultConfig()
	config.Address = consulAddr
	client, err := consul.NewClient(config)
	if err != nil {
		log.Printf("Error creating consul client for deregistration: %v", err)
		return
	}

	if err := client.Agent().ServiceDeregister("sentinel-controller"); err != nil {
		log.Printf("Error deregistering gRPC service: %v", err)
	}

	if err := client.Agent().ServiceDeregister("sentinel-controller-http"); err != nil {
		log.Printf("Error deregistering HTTP service: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func mustAtoi(s string) int {
	var i int
	fmt.Sscanf(s, "%d", &i)
	return i
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return "127.0.0.1"
}
