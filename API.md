# HTTP API Documentation

## Endpoints

### Health Check

**GET /api/v1/health**

Check if the service is healthy and database is accessible.

**Response**
```json
{
  "status": "healthy",
  "database": "connected"
}
```

**Status Codes**
- `200 OK`: Service is healthy
- `503 Service Unavailable`: Database connection failed

### List All Hosts

**GET /api/v1/hosts**

Retrieve a list of all hosts in the cluster.

**Response**
```json
{
  "hosts": [
    {
      "id": 1,
      "hostname": "server-01",
      "ip": "192.168.1.100",
      "uptime_seconds": 3600,
      "cpu_cores": 8,
      "total_memory_bytes": 17179869184,
      "total_storage_bytes": 214748364800,
      "last_seen": "2025-12-01T10:30:00Z",
      "online": true,
      "created_at": "2025-12-01T09:00:00Z",
      "updated_at": "2025-12-01T10:30:00Z"
    }
  ],
  "count": 1
}
```

**Status Codes**
- `200 OK`: Success

### Get Host Details

**GET /api/v1/hosts/{hostname}**

Retrieve detailed information about a specific host, including usage history.

**Path Parameters**
- `hostname` (required): The hostname of the target host

**Query Parameters**
- `limit` (optional): Number of usage records to return (default: 100, max: 1000)

**Example**
```bash
GET /api/v1/hosts/server-01?limit=50
```

**Response**
```json
{
  "host": {
    "id": 1,
    "hostname": "server-01",
    "ip": "192.168.1.100",
    "uptime_seconds": 3600,
    "cpu_cores": 8,
    "total_memory_bytes": 17179869184,
    "total_storage_bytes": 214748364800,
    "last_seen": "2025-12-01T10:30:00Z",
    "online": true,
    "created_at": "2025-12-01T09:00:00Z",
    "updated_at": "2025-12-01T10:30:00Z"
  },
  "usage": [
    {
      "id": 123,
      "host_id": 1,
      "timestamp": "2025-12-01T10:30:00Z",
      "cpu_percent": 45.5,
      "used_memory_bytes": 8589934592,
      "used_storage_bytes": 107374182400
    }
  ]
}
```

**Status Codes**
- `200 OK`: Success
- `404 Not Found`: Host not found
- `400 Bad Request`: Invalid hostname

### Get Cluster Statistics

**GET /api/v1/stats**

Retrieve aggregate statistics for the entire cluster.

**Response**
```json
{
  "total_hosts": 10,
  "online_hosts": 8,
  "offline_hosts": 2,
  "total_cpu_cores": 64,
  "total_memory_bytes": 137438953472,
  "total_storage_bytes": 1099511627776,
  "avg_cpu_percent": 42.3
}
```

**Fields**
- `total_hosts`: Total number of hosts ever registered
- `online_hosts`: Number of currently online hosts
- `offline_hosts`: Number of currently offline hosts
- `total_cpu_cores`: Sum of CPU cores across online hosts
- `total_memory_bytes`: Sum of total memory across online hosts
- `total_storage_bytes`: Sum of total storage across online hosts
- `avg_cpu_percent`: Average CPU usage across all hosts (last 5 minutes)

**Status Codes**
- `200 OK`: Success

## Script Management

### List All Scripts

**GET /api/v1/scripts**

Retrieve all scripts that have been created.

**Response**
```json
{
  "scripts": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "update-packages",
      "content": "#!/bin/bash\napt-get update && apt-get upgrade -y",
      "sha256_hash": "a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3",
      "created_at": "2025-12-05T10:00:00Z"
    }
  ],
  "count": 1
}
```

**Status Codes**
- `200 OK`: Success

### Create Script

**POST /api/v1/scripts**

Create a new script and distribute it to hosts. Scripts are sent to all online hosts, or only to hosts with specific tags if provided.

**Request Body**
```json
{
  "name": "update-packages",
  "content": "#!/bin/bash\napt-get update && apt-get upgrade -y",
  "tags": ["production", "web-server"]
}
```

**Parameters**
- `name` (required): Human-readable name for the script
- `content` (required): Script content (bash/shell script)
- `tags` (optional): Array of tags to filter target hosts. If empty/omitted, script is sent to all online hosts.

**Script Execution**
- Scripts are distributed immediately to matching online hosts via the gRPC stream
- Each host tracks executed scripts by SHA256 hash and only runs each unique script once
- Execution happens asynchronously on outpost agents
- Results (exit code, stdout, stderr) are reported back to the collector

**Response** `201 Created`
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "update-packages",
  "content": "#!/bin/bash\napt-get update && apt-get upgrade -y",
  "sha256_hash": "a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3",
  "created_at": "2025-12-05T10:00:00Z"
}
```

**Status Codes**
- `201 Created`: Script created and distribution initiated
- `400 Bad Request`: Missing name or content

### Get Script Details

**GET /api/v1/scripts/{script_id}**

Retrieve script details and execution history across all hosts.

**Path Parameters**
- `script_id` (required): UUID of the script

**Response**
```json
{
  "script": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "update-packages",
    "content": "#!/bin/bash\napt-get update && apt-get upgrade -y",
    "sha256_hash": "a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3",
    "created_at": "2025-12-05T10:00:00Z"
  },
  "executions": [
    {
      "id": 1,
      "script_id": "550e8400-e29b-41d4-a716-446655440000",
      "host_id": 1,
      "hostname": "server-01",
      "sha256_hash": "a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3",
      "exit_code": 0,
      "stdout": "Reading package lists...\nDone\n",
      "stderr": "",
      "executed_at": "2025-12-05T10:01:00Z"
    }
  ]
}
```

**Status Codes**
- `200 OK`: Success
- `404 Not Found`: Script not found

### Delete Script

**DELETE /api/v1/scripts/{script_id}**

Delete a script and its execution history.

**Path Parameters**
- `script_id` (required): UUID of the script

**Response**
```json
{
  "message": "Script deleted successfully"
}
```

**Status Codes**
- `200 OK`: Success
- `404 Not Found`: Script not found (but deletion succeeds anyway)

## Tag Management

### List All Tags

**GET /api/v1/tags**

Retrieve all tags that have been created.

**Response**
```json
{
  "tags": [
    {
      "id": 1,
      "name": "production",
      "created_at": "2025-12-01T00:00:00Z"
    },
    {
      "id": 2,
      "name": "web-server",
      "created_at": "2025-12-01T00:00:00Z"
    }
  ],
  "count": 2
}
```

**Status Codes**
- `200 OK`: Success

### Add Tag to Host

**POST /api/v1/hosts/tags**

Associate a tag with a host. Tags are automatically created if they don't exist.

**Request Body**
```json
{
  "hostname": "server-01",
  "tag": "production"
}
```

**Response**
```json
{
  "message": "Tag added successfully"
}
```

**Status Codes**
- `200 OK`: Success
- `400 Bad Request`: Missing hostname or tag

### Remove Tag from Host

**DELETE /api/v1/hosts/tags**

Remove a tag association from a host.

**Request Body**
```json
{
  "hostname": "server-01",
  "tag": "production"
}
```

**Response**
```json
{
  "message": "Tag removed successfully"
}
```

**Status Codes**
- `200 OK`: Success
- `400 Bad Request`: Missing hostname or tag

## Error Responses

All endpoints may return the following error responses:

**405 Method Not Allowed**
```
Method not allowed
```

**500 Internal Server Error**
```
Internal server error
```

## Service Discovery

The HTTP API is automatically registered with Consul under the name `node-metrics-collector-http`.

**Query Consul**
```bash
curl http://consul:8500/v1/catalog/service/node-metrics-collector-http
```