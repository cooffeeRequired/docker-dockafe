.PHONY: fmt vet test build run clean install shots

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 1.0.0)
LDFLAGS := -s -w -X github.com/cooffeeRequired/dockafe/internal/ui.AppVersion=$(VERSION)

fmt:
	gofmt -w ./internal ./main.go ./cmd

vet:
	go vet ./...

test:
	go test ./...

build:
	go build -ldflags="$(LDFLAGS)" -o dockafe .
	@echo "Built dockafe $(VERSION) — if TUI is running, press q and start again."

run: build
	./dockafe

demo: build
	./dockafe --demo

shots:
	go run ./cmd/genshots docs/assets
	python3 scripts/ansi_to_png.py docs/assets
	rm -f docs/assets/*.ansi
	@echo "Updated docs/assets/screenshot-*.png (demo data)"

install: build
	install -m 755 dockafe "$$(go env GOPATH)/bin/dockafe"
	@echo "Installed to $$(go env GOPATH)/bin/dockafe"

clean:
	rm -f dockafe docker-tui docs/assets/*.ansi
