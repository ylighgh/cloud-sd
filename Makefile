BINARY := cloud-sd

.PHONY: test build run tidy

test:
	go test ./...

build:
	mkdir -p bin
	go build -o bin/$(BINARY) ./cmd/cloud-sd

run:
	go run ./cmd/cloud-sd -config examples/config.yaml

tidy:
	go mod tidy

