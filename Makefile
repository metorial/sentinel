.PHONY: all proto build build-commander build-outpost build-cli docker clean install-outpost install-cli test test-unit test-integration test-coverage

all: proto build

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/metrics.proto

build: build-commander build-outpost build-cli

build-commander:
	CGO_ENABLED=0 go build -o bin/commander ./cmd/commander

build-outpost:
	CGO_ENABLED=0 go build -o bin/outpost ./cmd/outpost

build-cli:
	CGO_ENABLED=0 go build -o bin/nodectl ./cmd/nodectl

docker:
	docker build -t command-core-commander:latest -f Dockerfile.commander .

install-outpost: build-outpost
	sudo cp bin/outpost /usr/local/bin/node-outpost
	sudo cp deploy/node-outpost.service /etc/systemd/system/
	sudo systemctl daemon-reload
	sudo systemctl enable node-outpost
	sudo systemctl start node-outpost

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
