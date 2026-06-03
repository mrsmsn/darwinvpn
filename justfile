set shell := ["bash", "-cu"]

# Show available recipes.
default:
    @just --list

# Build the darwinvpn binary into ./darwinvpn (cgo enabled).
build:
    CGO_ENABLED=1 go build -o darwinvpn ./cmd/darwinvpn

# Build with a version string injected via ldflags.
# Usage: just build-versioned v0.1.0
build-versioned VERSION:
    CGO_ENABLED=1 go build \
      -ldflags="-X 'github.com/mrsmsn/darwinvpn/internal/cli.version={{VERSION}}'" \
      -o darwinvpn ./cmd/darwinvpn

# Run all tests with the race detector.
test:
    go test -race -count=1 ./...

# Run go vet across the module.
vet:
    go vet ./...

# Format the codebase.
fmt:
    gofmt -w -s .

# Static analysis via go vet + tidy check.
lint: vet
    go mod tidy -diff

# Remove build artifacts.
clean:
    rm -f darwinvpn coverage.out
    go clean ./...
