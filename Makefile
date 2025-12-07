.PHONY: all proto build build-commander build-agent build-cli docker clean install-agent install-cli test test-unit test-integration test-coverage

all: proto build

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/metrics.proto

build: build-controller build-agent build-cli

build-controller:
	CGO_ENABLED=0 go build -o bin/controller ./cmd/commander

build-agent:
	CGO_ENABLED=0 go build -o bin/agent ./cmd/agent

build-cli:
	CGO_ENABLED=0 go build -o bin/nodectl ./cmd/nodectl

docker:
	docker build -t sentinel-controller:latest -f Dockerfile.controller .

install-agent: build-agent
	sudo cp bin/agent /usr/local/bin/node-agent
	sudo cp deploy/node-agent.service /etc/systemd/system/
	sudo systemctl daemon-reload
	sudo systemctl enable node-agent
	sudo systemctl start node-agent

install-cli: build-cli
	sudo cp bin/nodectl /usr/local/bin/nodectl

test: test-unit test-integration

test-unit:
	CGO_ENABLED=0 go test -v -short ./internal/...

test-integration:
	CGO_ENABLED=0 go test -v ./test/integration/...

test-coverage:
	CGO_ENABLED=0 go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean:
	rm -rf bin/
	rm -f proto/*.pb.go
	rm -f coverage.out coverage.html
