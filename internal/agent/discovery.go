package agent

import (
	"fmt"
	"log"
	"time"

	consul "github.com/hashicorp/consul/api"
)

type ServiceDiscovery struct {
	consulAddr string
	client     *consul.Client
}

func NewServiceDiscovery(consulAddr string) (*ServiceDiscovery, error) {
	config := consul.DefaultConfig()
	config.Address = consulAddr

	client, err := consul.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("create consul client: %w", err)
	}

	return &ServiceDiscovery{
		consulAddr: consulAddr,
		client:     client,
	}, nil
}

func (sd *ServiceDiscovery) DiscoverCommander() (string, error) {
	services, _, err := sd.client.Health().Service("sentinel-controller", "", true, nil)
	if err != nil {
		return "", fmt.Errorf("query consul: %w", err)
	}

	if len(services) == 0 {
		return "", fmt.Errorf("no healthy commander services found")
	}

	service := services[0]
	addr := service.Service.Address
	if addr == "" {
		addr = service.Node.Address
	}

	return fmt.Sprintf("%s:%d", addr, service.Service.Port), nil
}

func (sd *ServiceDiscovery) WatchCommander() <-chan string {
	addrChan := make(chan string, 1)

	go func() {
		var lastAddr string
		for {
			addr, err := sd.DiscoverCommander()
			if err != nil {
				log.Printf("Discovery failed: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			if addr != lastAddr {
				log.Printf("Discovered commander at: %s", addr)
				addrChan <- addr
				lastAddr = addr
			}

			time.Sleep(10 * time.Second)
		}
	}()

	return addrChan
}
