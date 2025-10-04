# Default recipe - shows interactive chooser when just running 'just'
default:
    @just --choose

# URLs for downloaded data
SHAKESPERT_DB_URL := "https://github.com/andreweick/prospero-data/releases/download/v0.0.1/shakespert.db"
TOPTEN_DATA_URL := "https://github.com/andreweick/prospero-data/releases/download/v0.0.1/topten.json.age"

# Exit without doing anything (for chooser menu)
quit:
    @echo "👋 Exiting..."

# Format code using gofumpt
fmt:
    go tool gofumpt -w .

# Download Shakespeare database if it doesn't exist
download-shakespert-db:
    #!/usr/bin/env bash
    mkdir -p assets/data
    if [ ! -f assets/data/shakespert.db ]; then
        echo "📚 Downloading Shakespeare database..."
        curl -L -o assets/data/shakespert.db {{SHAKESPERT_DB_URL}}
        echo "✅ Downloaded shakespert.db"
    fi

# Download top ten data if it doesn't exist
download-topten-data:
    #!/usr/bin/env bash
    mkdir -p assets/data
    if [ ! -f assets/data/topten.json.age ]; then
        echo "🔟 Downloading top ten data..."
        curl -L -o assets/data/topten.json.age {{TOPTEN_DATA_URL}}
        echo "✅ Downloaded topten.json.age"
    fi

# Download all data files
download-data: download-shakespert-db download-topten-data
    @echo "📦 All data files downloaded"

# Force re-download of Shakespeare database
update-shakespert-db:
    @rm -f assets/data/shakespert.db
    @just download-shakespert-db

# Force re-download of top ten data
update-topten-data:
    @rm -f assets/data/topten.json.age
    @just download-topten-data

# Force re-download of all data
update-data:
    @rm -f assets/data/shakespert.db assets/data/topten.json.age
    @just download-data

# Build the prospero binary
build: download-data fmt
    go build -o bin/prospero ./cmd/prospero

# Run the prospero binary (depends on build)
run *args: build
    ./bin/prospero {{args}}

# Start both HTTP and SSH servers (default ports: 8080, 2222)
serve *args: build
    ./bin/prospero serve {{args}}

# Run the sops
sops:
    go tool sops --version

# Start servers on custom ports
serve-custom http_port ssh_port: build
    ./bin/prospero serve --http-port {{http_port}} --ssh-port {{ssh_port}}

# Display a random top-ten list in CLI
top-ten *args: build
    ./bin/prospero top-ten {{args}}

# MCP server
mcp: build
    ./bin/prospero mcp

# Shakespeare CLI commands
shakespert-works *args: build
    ./bin/prospero shakespert works {{args}}

shakespert-work work_id: build
    ./bin/prospero shakespert work {{work_id}}

shakespert-genres: build
    ./bin/prospero shakespert genres

# Quick demo of all features
demo: build
    @echo "🎩 Prospero Demo"
    @echo "=================="
    @echo ""
    @echo "📚 Shakespeare's Works (first 10):"
    @./bin/prospero shakespert works | head -12
    @echo ""
    @echo "📖 Hamlet Details:"
    @./bin/prospero shakespert work hamlet
    @echo ""
    @echo "🎭 Top 10 List:"
    @./bin/prospero top-ten --ascii

# Run tests
test:
    go test -v ./...

# Clean build artifacts
clean:
    rm -rf bin/

# Build for Magic Container (linux/amd64)
build-container:
    GOOS=linux GOARCH=amd64 go build -o bin/prospero-linux ./cmd/prospero

# Build containers
container-magic:
    podman build -f deploy/magic-container/Containerfile -t localhost/prospero:latest .

container-bootc:
    podman build -f deploy/bootc/Containerfile -t localhost/prospero-bootc:latest .

containers: container-magic container-bootc

# Run Magic Container locally
container-run-magic:
    podman run -p 8080:8080 -p 2222:2222 localhost/prospero:latest

# Development helpers - set AGE_ENCRYPTION_PASSWORD if needed
run-with-password: build
    #!/usr/bin/env bash
    if [ -z "$AGE_ENCRYPTION_PASSWORD" ]; then
        echo "AGE_ENCRYPTION_PASSWORD not set. Please enter password:"
        read -s password
        export AGE_ENCRYPTION_PASSWORD="$password"
    fi
    ./bin/prospero top-ten

serve-with-passwor: build
    #!/usr/bin/env bash
    if [ -z "$AGE_ENCRYPTION_PASSWORD" ]; then
        echo "AGE_ENCRYPTION_PASSWORD not set. Please enter password:"
        read -s password
        export AGE_ENCRYPTION_PASSWORD="$password"
    fi
    ./bin/prospero serve

# API Testing helpers (assumes server running on localhost:8080)
test-health:
    curl -s http://localhost:8080/health | jq .

test-api-json:
    curl -s http://localhost:8080/api/top-ten | jq .

test-api-ascii:
    curl -s http://localhost:8080/api/top-ten?format=ascii

# SSH tests (assumes server running on localhost:2222)
test-ssh-help:
    ssh localhost -p 2222

test-ssh-topten:
    ssh localhost -p 2222 topten

test-ssh-shakespert:
    ssh localhost -p 2222 shakespert works | head -10

test-ssh-all: test-ssh-help test-ssh-topten test-ssh-shakespert

# Test all endpoints
test-all-endpoints: test-health test-api-json test-api-ascii test-shakespert-api
    @echo "✅ All HTTP endpoints tested"

# Test everything (requires server running)
test-everything: test-all-endpoints test-ssh-all
    @echo "🎉 All interfaces tested!"

# Generate sqlc code
sqlc-generate:
    $HOME/go/bin/sqlc generate

# Shakespert API tests
test-shakespert-works:
    curl -s http://localhost:8080/api/shakespert/works | jq .

test-shakespert-work:
    curl -s http://localhost:8080/api/shakespert/works/hamlet | jq .

test-shakespert-genres:
    curl -s http://localhost:8080/api/shakespert/genres | jq .

test-shakespert-api: test-shakespert-works test-shakespert-work test-shakespert-genres
    @echo "✅ Shakespeare HTTP endpoints tested"

# Development commands - Extract embedded data for local development
extract-all: build
    ./bin/prospero dev extract all

extract-topten: build
    ./bin/prospero dev extract topten

extract-shakespert: build
    ./bin/prospero dev extract shakespert

extract-secrets: build
    ./bin/prospero dev extract secrets

# Pack modified data for embedding
pack-shakespert: build
    ./bin/prospero dev pack shakespert --force

# Rotate encryption keys (requires PREVIOUS_AGE_ENCRYPTION_PASSWORD)
rotate-key: build
    ./bin/prospero dev rotate-key

rotate-key-dry-run: build
    ./bin/prospero dev rotate-key --dry-run

# Check that all embedded data fits within Magic Container limits
check-size:
    #!/usr/bin/env sh
    set -e

    echo "Checking embedded data sizes for Magic Container deployment..."
    echo "Limit: 10 GB ephemeral storage"
    echo ""

    total_bytes=0

    # Function to get file size (cross-platform)
    get_size() {
        if [ "$(uname)" = "Darwin" ]; then
            stat -f%z "$1" 2>/dev/null || echo 0
        else
            stat -c%s "$1" 2>/dev/null || echo 0
        fi
    }

    # Check binary size
    if [ -f bin/prospero ]; then
        bin_size=$(get_size bin/prospero)
        total_bytes=$((total_bytes + bin_size))
        printf "Binary:        %10.1f MB  bin/prospero\n" $(echo "$bin_size" | awk '{print $1/1024/1024}')
    fi

    # Check embedded data files
    for file in assets/data/*.gz assets/data/*.age assets/data/*.sql.gz assets/data/*.db; do
        if [ -f "$file" ]; then
            size=$(get_size "$file")
            total_bytes=$((total_bytes + size))
            printf "Embedded:      %10.1f MB  %s\n" $(echo "$size" | awk '{print $1/1024/1024}') "$file"
        fi
    done

    # Add estimated runtime overhead (Go runtime, decompressed data in memory)
    # Assume 2x for decompressed data + runtime overhead
    runtime_overhead=$((total_bytes * 2))
    total_with_overhead=$((total_bytes + runtime_overhead))

    echo "----------------------------------------"
    printf "Total embedded: %9.1f MB\n" $(echo "$total_bytes" | awk '{print $1/1024/1024}')
    printf "Runtime estimate: %7.1f MB (includes decompressed data + overhead)\n" $(echo "$total_with_overhead" | awk '{print $1/1024/1024}')
    echo ""

    # Check against limit (10 GB = 10737418240 bytes)
    limit=10737418240
    if [ "$total_with_overhead" -gt "$limit" ]; then
        echo "❌ ERROR: Estimated size exceeds 10 GB Magic Container limit!"
        exit 1
    else
        percent=$(echo "$total_with_overhead $limit" | awk '{printf "%.1f", ($1/$2)*100}')
        echo "✅ PASS: Using $percent% of 10 GB limit"

        if [ "$total_with_overhead" -gt 5368709120 ]; then
            echo "⚠️  WARNING: Over 50% of limit - consider optimizing"
        fi
    fi

# Build and check size in one command
build-check: build check-size
    @echo "Build complete and size verified"

# Quick start guide
help:
    @echo "🎩 Prospero - Quick Commands"
    @echo "============================="
    @echo ""
    @echo "🔧 Building:"
    @echo "  just build              # Build the binary"
    @echo "  just demo               # Quick demo of all features"
    @echo ""
    @echo "🖥️  CLI Commands:"
    @echo "  just top-ten            # Random David Letterman list"
    @echo "  just shakespert-works   # List Shakespeare's works"
    @echo "  just shakespert-genres  # List available genres"
    @echo ""
    @echo "🌐 Server & Testing:"
    @echo "  just serve              # Start HTTP (8080) + SSH (2222) servers"
    @echo "  just test-everything    # Test all endpoints (requires server)"
    @echo ""
    @echo "🛠️  Development:"
    @echo "  just extract-all        # Extract all embedded data for development"
    @echo "  just pack-shakespert    # Recompress shakespert database after changes"
    @echo "  just rotate-key         # Rotate age encryption keys"
    @echo ""
    @echo "📚 More commands: just --list"
