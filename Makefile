BINARY := prometheus-cloud-sd

.PHONY: test build run tidy

test:
	go test ./...

build:
	mkdir -p bin
	go build -o bin/$(BINARY) ./cmd/prometheus-cloud-sd

run:
	go run ./cmd/prometheus-cloud-sd -config examples/config.yaml

tidy:
	go mod tidy

