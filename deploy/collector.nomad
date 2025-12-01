job "node-metrics-collector" {
  datacenters = ["dc1"]
  type        = "service"

  group "collector" {
    count = 1

    network {
      port "grpc" {
        static = 9090
      }
      port "http" {
        static = 8080
      }
    }

    volume "data" {
      type      = "host"
      source    = "node-metrics-data"
      read_only = false
    }

    task "collector" {
      driver = "docker"

      config {
        image = "node-metrics-collector:latest"
        ports = ["grpc", "http"]

        mount {
          type   = "bind"
          source = "local"
          target = "/data"
        }
      }

      volume_mount {
        volume      = "data"
        destination = "/data"
      }

      env {
        PORT             = "${NOMAD_PORT_grpc}"
        HTTP_PORT        = "${NOMAD_PORT_http}"
        DB_PATH          = "/data/metrics.db"
        CONSUL_HTTP_ADDR = "${attr.unique.network.ip-address}:8500"
      }

      resources {
        cpu    = 500
        memory = 512
      }

      service {
        name = "node-metrics-collector"
        port = "grpc"
        tags = ["metrics", "collector", "grpc"]

        check {
          type     = "grpc"
          interval = "10s"
          timeout  = "5s"
        }
      }

      service {
        name = "node-metrics-collector-http"
        port = "http"
        tags = ["metrics", "collector", "http", "api"]

        check {
          type     = "http"
          path     = "/api/v1/health"
          interval = "10s"
          timeout  = "5s"
        }
      }
    }
  }
}
