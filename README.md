# Node Manager

A lightweight system for collecting and querying host metrics across a distributed fleet. Includes support for remote script execution and node tagging. Think of it as a simple monitoring and management solution where agents report system metrics to a central collector that you can query via CLI or API.

## Features

- **Metrics Collection** - CPU, memory, disk, and network usage
- **Script Execution** - Run bash scripts on nodes, with one-time execution guarantee
- **Node Tagging** - Organize nodes with tags for targeted script execution
- **HTTP API** - RESTful API for querying metrics and managing scripts
- **Service Discovery** - Automatic collector discovery via Consul (optional)
- **SQLite Storage** - Lightweight embedded database

## Components

**collector** - Central API server that receives metrics from outpost agents and stores them in SQLite. Provides HTTP endpoints for querying host information, managing scripts, and viewing execution results.

**outpost** - Agent that runs on each node to collect system metrics and execute scripts received from the collector.

**nodectl** - Command-line tool to interact with the collector API. Query health status, list hosts, view usage stats.

## Quick Start

### Option 1: Direct Connection (Simple)

Run the collector:

```bash
docker run -d -p 8080:8080 -p 9090:9090 \
  ghcr.io/metorial/fleet/node-manager/collector:latest
```

Deploy outpost agents on your nodes:

```bash
docker run -d \
  -e COLLECTOR_URL=collector.example.com:9090 \
  ghcr.io/metorial/fleet/node-manager/outpost:latest
```

### Option 2: Consul Service Discovery (Dynamic)

Run the collector with Consul registration:

```bash
docker run -d -p 8080:8080 -p 9090:9090 \
  -e CONSUL_HTTP_ADDR=consul.example.com:8500 \
  ghcr.io/metorial/fleet/node-manager/collector:latest
```

Deploy outpost agents:

```bash
docker run -d \
  -e CONSUL_HTTP_ADDR=consul.example.com:8500 \
  ghcr.io/metorial/fleet/node-manager/outpost:latest
```

**Note:** Either `COLLECTOR_URL` or `CONSUL_HTTP_ADDR` must be set for outposts. `COLLECTOR_URL` takes precedence if both are set.

## Using the CLI

Query metrics and cluster information:

```bash
# Check collector health
nodectl --server http://collector:8080 health

# List all hosts
nodectl --server http://collector:8080 hosts list

# Get detailed host info with tags
nodectl --server http://collector:8080 hosts get my-hostname

# View cluster statistics
nodectl --server http://collector:8080 stats
```

You can set `NODECTL_SERVER_URL` environment variable to avoid passing `--server` every time.

## Service Discovery with Consul

The Node Manager integrates with Consul for automatic service discovery and health checking.

### How It Works

1. **Collector Registration**: When started with `CONSUL_HTTP_ADDR`, the collector registers itself with Consul:
   - Service name: `node-metrics-collector` (gRPC on port 9090)
   - HTTP API: `node-metrics-collector-http` (REST on port 8080)
   - Health checks on both endpoints

2. **Outpost Discovery**: Outposts query Consul for the `node-metrics-collector` service
   - Automatic reconnection if collector address changes
   - Polling interval: 10 seconds

3. **Benefits**:
   - No hardcoded collector addresses
   - Automatic failover if collector moves
   - Built-in health monitoring
   - Works in dynamic environments (Kubernetes, cloud platforms)

### Environment Variables

**Collector:**
- `PORT` - gRPC port (default: 9090)
- `HTTP_PORT` - HTTP API port (default: 8080)
- `DB_PATH` - SQLite database path (default: /data/metrics.db)
- `CONSUL_HTTP_ADDR` - Consul address for registration (optional)

**Outpost:**
- `COLLECTOR_URL` - Direct collector address (e.g., `collector:9090`)
- `CONSUL_HTTP_ADDR` - Consul address for service discovery
- **Note:** Either `COLLECTOR_URL` or `CONSUL_HTTP_ADDR` must be set

## Architecture

```
┌─────────────┐
│    Consul   │ ◄─── Service Registration
└──────┬──────┘
       │ Service Discovery
       │
┌──────▼──────┐      gRPC Stream        ┌──────────────┐
│   Outpost   ├─────────────────────────►   Collector  │
│    Agent    │   Metrics + Scripts     │   (Server)   │
└─────────────┘   Results               └──────┬───────┘
                                               │
                                        ┌──────▼───────┐
                                        │   HTTP API   │
                                        └──────┬───────┘
                                               │
                                        ┌──────▼───────┐
                                        │  (nodectl)   │
                                        └──────────────┘
```

## License

Licensed under Apache License 2.0. See [LICENSE](LICENSE) file for details.
