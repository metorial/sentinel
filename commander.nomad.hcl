job "commander" {
  datacenters = ["dc1"]
  type        = "service"

  group "commander" {
    count = 1

    network {
      port "grpc" {
        static = 9090
        to     = 9090
      }
      port "http" {
        static = 8080
        to     = 8080
      }
    }

    volume "commander-data" {
      type      = "host"
      source    = "commander-data"
      read_only = false
    }

    task "commander" {
      driver = "docker"

      config {
        image = "ghcr.io/metorial/command-core-commander:latest"
        ports = ["grpc", "http"]
      }

      volume_mount {
        volume      = "commander-data"
        destination = "/data"
        read_only   = false
      }

      template {
        data = <<EOF
CONSUL_HTTP_ADDR="{{ env "attr.consul.address" }}"
EOF
        destination = "local/consul.env"
        env         = true
      }

      env {
        PORT        = "${NOMAD_PORT_grpc}"
        HTTP_PORT   = "${NOMAD_PORT_http}"
        DB_PATH     = "/data/metrics.db"
      }

      resources {
        memory = 256
      }

      service {
        name = "command-core-commander"
        port = "grpc"
        tags = ["grpc", "metrics"]

        check {
          type     = "tcp"
          port     = "grpc"
          interval = "10s"
          timeout  = "5s"
        }
      }

      service {
        name = "command-core-commander-http"
        port = "http"
        tags = ["http", "api"]

        check {
          type     = "http"
          path     = "/api/v1/health"
          port     = "http"
          interval = "10s"
          timeout  = "5s"
        }
      }
    }
  }
}
