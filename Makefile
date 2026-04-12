.PHONY: dev build test lint fmt clean release-snapshot ui-install ui-dev ui-build

BINARY_NAME := aetronyx
BUILD_DIR   := dist
GO_FLAGS    := -trimpath

## dev: run backend with hot reload (requires air or similar) + frontend dev server
dev:
	@echo "Starting dev environment..."
	@echo "Backend: go run ."
	@echo "Frontend: cd ui && npm run dev"
	@echo "Full hot-reload setup will be documented in M1."
	go run .

## build: compile the Go binary (embeds ui/dist if present)
build:
	@mkdir -p $(BUILD_DIR)
	go build $(GO_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .

## test: run all Go tests with race detector
test:
	go test -race -count=1 ./...

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## fmt: format Go source files
fmt:
	gofmt -w .
	goimports -w .

## clean: remove build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -rf ui/dist ui/out ui/.next

## release-snapshot: build snapshot release with goreleaser
release-snapshot:
	goreleaser release --snapshot --clean

## ui-install: install frontend dependencies
ui-install:
	cd ui && npm install

## ui-dev: start Next.js development server
ui-dev:
	cd ui && npm run dev

## ui-build: build Next.js static export for embedding
ui-build:
	cd ui && npm run build
