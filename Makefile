.PHONY: all build install clean test

BINARY_NAME=docs_organiser
GOPATH=$(shell go env GOPATH)

all: build

build:
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) .

install: build
	@echo "Installing $(BINARY_NAME) to $(GOPATH)/bin..."
	cp $(BINARY_NAME) $(GOPATH)/bin/

clean:
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME)

test:
	@echo "Running tests..."
	go test ./...
