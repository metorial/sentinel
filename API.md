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