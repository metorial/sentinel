# Sentinel

A lightweight system for collecting and querying host metrics across a distributed fleet. Includes support for remote script execution and node tagging. Think of it as a simple monitoring and management solution where agents report system metrics to a central controller that you can query via CLI or API.

## Features

- **Metrics Collection** - CPU, memory, disk, and network usage
- **Script Execution** - Run bash scripts on nodes, with one-time execution guarantee
- **Node Tagging** - Organize nodes with tags for targeted script execution
- **HTTP API** - RESTful API for querying metrics and managing scripts
- **Service Discovery** - Automatic commander discovery via Consul (optional)
- **SQLite Storage** - Lightweight embedded database

## Components

**controller** - Central API server that receives metrics from agent agents and stores them in SQLite. Provides HTTP endpoints for querying host information, managing scripts, and viewing execution results.

**agent** - Agent that runs on each node to collect system metrics and execute scripts received from the controller.

**nodectl** - Command-line tool to interact with the controller API. Query health status, list hosts, view usage stats.

## Quick Start

### Option 1: Direct Connection (Simple)

Run the controller:

```bash
docker run -d -p 8080:8080 -p 9090:9090 \
  ghcr.io/metorial/sentinel-controller:latest
```

Deploy agent agents on your nodes:

```bash
docker run -d \
  -e COLLECTOR_URL=controller.example.com:9090 \
  ghcr.io/metorial/sentinel-agent:latest
```

### Option 2: Consul Service Discovery

Run the controller with Consul registration:

```bash
docker run -d -p 8080:8080 -p 9090:9090 \
  -e CONSUL_HTTP_ADDR=consul.example.com:8500 \
  ghcr.io/metorial/sentinel-controller:latest
```

Deploy agent agents:

```bash
docker run -d \
  -e CONSUL_HTTP_ADDR=consul.example.com:8500 \
  ghcr.io/metorial/sentinel-agent:latest
```

**Note:** Either `COLLECTOR_URL` or `CONSUL_HTTP_ADDR` must be set for agents. `COLLECTOR_URL` takes precedence if both are set.

## Using the CLI

Query metrics and cluster information:

```bash
# Check controller health
nodectl --server http://controller:8080 health

# List all hosts
nodectl --server http://controller:8080 hosts list

# Get detailed host info with tags
nodectl --server http://controller:8080 hosts get my-hostname

# View cluster statistics
nodectl --server http://controller:8080 stats
```

You can set `NODECTL_SERVER_URL` environment variable to avoid passing `--server` every time.

## Service Discovery with Consul

Sentinel integrates with Consul for automatic service discovery and health checking.

### How It Works

1. **Collector Registration**: When started with `CONSUL_HTTP_ADDR`, the controller registers itself with Consul:
   - Service name: `sentinel-controller` (gRPC on port 9090)
   - HTTP API: `sentinel-controller-http` (REST on port 8080)
   - Health checks on both endpoints

2. **agent Discovery**: agents query Consul for the `sentinel-controller` service
   - Automatic reconnection if controller address changes
   - Polling interval: 10 seconds

3. **Benefits**:
   - No hardcoded controller addresses
   - Automatic failover if controller moves
   - Built-in health monitoring
   - Works in dynamic environments (Kubernetes, cloud platforms)

### Environment Variables

**Collector:**
- `PORT` - gRPC port (default: 9090)
- `HTTP_PORT` - HTTP API port (default: 8080)
- `DB_PATH` - SQLite database path (default: /data/metrics.db)
- `CONSUL_HTTP_ADDR` - Consul address for registration (optional)

**agent:**
- `COLLECTOR_URL` - Direct controller address (e.g., `controller:9090`)
- `CONSUL_HTTP_ADDR` - Consul address for service discovery
- **Note:** Either `COLLECTOR_URL` or `CONSUL_HTTP_ADDR` must be set

## Architecture

```mermaid
graph TB
  Consul[Consul<br/>Service Registry]
  Controller[Controller Server<br/>gRPC + HTTP]
  agents[agent Agents<br/>Nodes 1..N]
  CLI[nodectl CLI]
  DB[(SQLite DB)]

  Controller --> Consul
  Consul --> agents

  agents -->|gRPC Stream| Controller
  Controller -->|Script Commands| agents

  Controller <-->|R/W| DB
  CLI -->|REST API| Controller

  style Controller fill:#4a9eff,stroke:#333,stroke-width:2px,color:#fff
  style Consul fill:#ca2171,stroke:#333,stroke-width:2px,color:#fff
  style DB fill:#669944,stroke:#333,stroke-width:2px,color:#fff
  style CLI fill:#f0ad4e,stroke:#333,stroke-width:2px,color:#000
```

## License

Licensed under Apache License 2.0. See [LICENSE](LICENSE) file for details.
