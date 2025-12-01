# Node Manager

A lightweight system for collecting and querying host metrics across a distributed fleet. Think of it as a simple monitoring solution where agents report system metrics to a central collector that you can query via CLI or API.

## Components

**collector** - Central API server that receives metrics from outpost agents and stores them in SQLite. Provides HTTP endpoints for querying host information and usage statistics.

**outpost** - Agent that runs on each node to collect system metrics (CPU, memory, disk, network) and reports them to the collector via gRPC.

**nodectl** - Command-line tool to interact with the collector API. Query health status, list hosts, view usage stats, and get detailed information about specific hosts.

## Usage

Run the collector:

```bash
docker run -d -p 8080:8080 -p 9090:9090 ghcr.io/metorial/fleet/node-manager/collector:latest
```

Deploy outpost agents on your nodes:

```bash
docker run -d -e COLLECTOR_URL=http://collector:8080 ghcr.io/metorial/fleet/node-manager/outpost:latest
```

Query the fleet:

```bash
# Check collector health
nodectl --server http://collector:8080 health

# List all hosts
nodectl --server http://collector:8080 hosts list

# Get detailed host info
nodectl --server http://collector:8080 hosts get my-hostname

# View cluster statistics
nodectl --server http://collector:8080 stats
```

You can set `NODECTL_SERVER_URL` environment variable to avoid passing `--server` every time.

## Building

```bash
make proto   # Generate protobuf files
make build   # Build all binaries to ./bin/
```

## License

Licensed under Apache License 2.0. See [LICENSE](LICENSE) file for details.
