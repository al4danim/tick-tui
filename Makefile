.PHONY: build install uninstall test clean run seed

BINARY    := bin/tick
INSTALL_DIR := $(HOME)/.local/bin

build:
	mkdir -p bin
	go build -o $(BINARY) ./cmd/tick

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/tick
	codesign --force --sign - $(INSTALL_DIR)/tick

uninstall:
	rm -f $(INSTALL_DIR)/tick

test:
	go test ./...

clean:
	rm -rf bin

run: build
	$(BINARY)

seed:
	go run ./cmd/seed --days 365 --avg 5 --out /tmp/tick-demo
