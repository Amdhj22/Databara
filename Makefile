BIN := bin/databara
PKG := ./cmd/databara

.PHONY: build run test lint tidy fmt clean

build:
	go build -o $(BIN) $(PKG)

run:
	go run $(PKG)

test:
	go test ./... -race -count=1

lint:
	golangci-lint run

tidy:
	go mod tidy

fmt:
	gofmt -s -w .
	goimports -w .

clean:
	rm -rf bin/
