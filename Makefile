.PHONY: setup build test bench lint clean

setup:
	go mod tidy
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

build:
	go build -o aether cmd/aether/main.go

test:
	go test -v -race ./...

bench:
	go test -bench=. ./...

lint:
	golangci-lint run

clean:
	rm -f aether
	rm -rf dist/
