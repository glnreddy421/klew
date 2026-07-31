.PHONY: build install install-cli test tidy desktop desktop-dev install-desktop-tools release-macos run-ctx run-view

VERSION ?= 0.1.0
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X github.com/glnreddy421/klew/internal/version.Version=$(VERSION) \
	-X github.com/glnreddy421/klew/internal/version.Commit=$(COMMIT) \
	-X github.com/glnreddy421/klew/internal/version.Date=$(DATE)

GOPATH_BIN := $(shell go env GOPATH)/bin
WAILS := $(GOPATH_BIN)/wails

build:
	go build -ldflags "$(LDFLAGS)" -o klew ./cmd/klew

install install-cli:
	go install -ldflags "$(LDFLAGS)" ./cmd/klew
	@echo "Installed klew to $(GOPATH_BIN)/klew"
	@test -d "$(GOPATH_BIN)" || (echo "Ensure $(GOPATH_BIN) is on your PATH" && exit 1)

install-desktop-tools:
	go install github.com/wailsapp/wails/v2/cmd/wails@latest
	@echo "Add to PATH if needed: export PATH=\"$(GOPATH_BIN):\$$PATH\""

desktop:
	cd cmd/klew-desktop && npm install --prefix frontend && npm run build --prefix frontend && $(WAILS) build

release-macos:
	VERSION=$(VERSION) ./scripts/package-macos.sh

desktop-dev:
	@test -x "$(WAILS)" || (echo "Wails not found. Run: make install-desktop-tools" && exit 1)
	cd cmd/klew-desktop && "$(WAILS)" dev

tidy:
	go mod tidy

test:
	go test ./...

run-ctx:
	go run ./cmd/klew ctx

run-view:
	go run ./cmd/klew view testdata/sample-bundle.json --no-tui

run-analyze:
	go run ./cmd/klew analyze -n prod payment --no-tui
