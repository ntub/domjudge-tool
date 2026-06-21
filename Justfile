# Justfile — task runner for domjudge-tool.
# Run `just` (or `just default`) to list available tasks.

set shell := ["bash", "-uc"]
set dotenv-load := false

# List all tasks.
default:
    @just --list --unsorted

# Install dev dependencies and git hooks. Idempotent.
setup:
    @echo "→ go mod download"
    @go mod download
    @echo "→ lefthook install"
    @if command -v lefthook >/dev/null 2>&1; then \
        lefthook install; \
    else \
        echo "⚠ lefthook not found. Install with: go install github.com/evilmartians/lefthook@latest"; \
    fi
    @echo "→ optional: install gofumpt + golangci-lint + trufflehog for full lint/security coverage"
    @if ! command -v gofumpt >/dev/null 2>&1; then \
        echo "  gofumpt:        go install mvdan.cc/gofumpt@latest"; \
    fi
    @if ! command -v golangci-lint >/dev/null 2>&1; then \
        echo "  golangci-lint:  brew install golangci-lint  # or see https://golangci-lint.run/welcome/install/"; \
    fi
    @if ! command -v trufflehog >/dev/null 2>&1; then \
        echo "  trufflehog:     curl -sSfL https://raw.githubusercontent.com/trufflesecurity/trufflehog/main/scripts/install.sh | sh -s -- -b /usr/local/bin"; \
    fi

# Build the CLI binary into ./bin.
build:
    @go build -o ./bin/domjudge-tool ./cmd/domjudge-tool

# Build the desktop app binary into ./bin.
build-desktop:
    @CGO_ENABLED=1 go build -tags desktop -o ./bin/domjudge-tool-app ./cmd/desktop

# Rebuild Tailwind CSS from templ templates (uses local npm install in frontend/).
# Output goes to internal/desktop/assets/tailwind.css — commit that file after running.
build-desktop-css:
    @./frontend/node_modules/.bin/tailwindcss -i ./frontend/input.css -o ./internal/desktop/assets/tailwind.css

# Regenerate templ Go files from *.templ sources.
generate-templates:
    @templ generate ./internal/desktop/templates/

# Run the CLI from source. Usage: `just run --help`
run *args:
    @go run ./cmd/domjudge-tool {{ args }}

# Run all tests.
test:
    @go test ./...

# Run tests with the race detector.
test-race:
    @go test -race ./...

# Generate HTML coverage report at coverage.html.
cover:
    @go test -coverprofile=coverage.out ./...
    @go tool cover -html=coverage.out -o coverage.html
    @echo "→ open coverage.html"

# Format Go sources. Prefers gofumpt; falls back to gofmt -s.
fmt:
    @if command -v gofumpt >/dev/null 2>&1; then \
        gofumpt -w .; \
    else \
        echo "⚠ gofumpt not installed; falling back to gofmt -s"; \
        gofmt -s -w .; \
    fi

# Run go vet on all packages.
vet:
    @go vet ./...

# Run golangci-lint. Errors out if the tool is not installed.
lint:
    @if ! command -v golangci-lint >/dev/null 2>&1; then \
        echo "✗ golangci-lint not installed. See: https://golangci-lint.run/welcome/install/"; \
        exit 1; \
    fi
    @golangci-lint run ./...

# Tidy go.mod / go.sum.
tidy:
    @go mod tidy

# Capture raw DOMjudge API v4 fixtures into tmp/raw_api_v4.
api-fixture-capture:
    @scripts/capture_api_fixtures.sh

# Scan source, docs, fixtures, and git history for committed secrets.
secret-scan:
    @scripts/check_secrets.sh

# Run local API fixture tooling checks.
api-fixture-check:
    @python3 -m unittest discover -s scripts -p '*_test.py'
    @python3 scripts/check_pii_api.py testdata/api/v4 testdata/web

# Run all repository privacy gates: generic secrets + project-specific PII.
privacy-check: secret-scan api-fixture-check

# Composite gate: fmt + vet + lint + privacy-check + test. Used by lefthook and CI.
check: fmt vet lint privacy-check test

# Regenerate the full CHANGELOG from git history.
changelog:
    @git-cliff -o CHANGELOG.md

# Preview unreleased changelog entries (does not write).
changelog-unreleased:
    @git-cliff --unreleased

# Cut a release: tag the commit and regenerate CHANGELOG.md.
# See docs/COMMITS.md and CONTRIBUTING.md for the manual workflow.
release version:
    @echo "→ tagging v{{ version }}"
    @git tag -a "v{{ version }}" -m "Release v{{ version }}"
    @git-cliff -o CHANGELOG.md
    @echo "→ next: review CHANGELOG.md, commit, then 'git push --follow-tags'"

# Validate goreleaser config (requires goreleaser ≥ v2).
goreleaser-check:
    @if ! command -v goreleaser >/dev/null 2>&1; then \
        echo "✗ goreleaser not installed. brew install goreleaser"; exit 1; \
    fi
    @goreleaser check --config .goreleaser.yaml

# Build a local snapshot (no publish, no tag required).
snapshot:
    @if ! command -v goreleaser >/dev/null 2>&1; then \
        echo "✗ goreleaser not installed. brew install goreleaser"; exit 1; \
    fi
    @goreleaser release --snapshot --clean

# Remove build / coverage artifacts.
clean:
    @rm -rf bin coverage.out coverage.html dist/
