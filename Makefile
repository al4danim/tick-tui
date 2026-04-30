.PHONY: build install test clean run

BINARY    := bin/tick
INSTALL_DIR := $(HOME)/.local/bin

build:
	mkdir -p bin
	go build -o $(BINARY) ./cmd/tick

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/tick

test:
	go test ./...

clean:
	rm -rf bin

run: build
	$(BINARY)
