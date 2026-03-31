.PHONY: build clean install test index embed ui serve

BINARY = bin/ledger
BUILD_FLAGS = -tags "fts5"

build:
	CGO_ENABLED=1 go build $(BUILD_FLAGS) -o $(BINARY) ./cmd/ledger/

clean:
	rm -f $(BINARY)
	rm -rf .ledger/

install: build
	cp $(BINARY) /usr/local/bin/ledger

# Quick workflow
index: build
	$(BINARY) index

embed: build
	$(BINARY) embed

search: build
	@test -n "$(Q)" || (echo "usage: make search Q=\"query\"" && exit 1)
	$(BINARY) search "$(Q)"

ui: build
	$(BINARY) ui

serve: build
	$(BINARY) serve

status: build
	$(BINARY) status

check: build
	$(BINARY) check

help:
	@echo "make build    — Build ledger binary"
	@echo "make install  — Install to /usr/local/bin"
	@echo "make clean    — Remove binary and DB"
	@echo "make index    — Build and index documents"
	@echo "make embed    — Build and generate embeddings"
	@echo "make ui       — Build and start web UI"
	@echo "make serve    — Build and start API server"
	@echo "make status   — Build and show status"
	@echo "make check    — Build and run contradiction check"
	@echo "make search Q=\"query\" — Build and search"
