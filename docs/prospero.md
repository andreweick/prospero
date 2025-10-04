# Prospero - Repository Structure and Architecture

A magical box of edge services and fun utilities for deployment on bunny.net Magic Containers.

## Repository Structure

```
prospero/
├── cmd/
│   └── prospero/
│       └── main.go          # Single binary entry point
│
├── internal/
│   ├── app/                 # Application logic
│   │   ├── cli/            # CLI command implementations
│   │   │   ├── root.go     # Root command setup
│   │   │   ├── topten.go   # topten subcommand
│   │   │   ├── serve.go    # serve subcommand
│   │   │   └── compress.go # compress subcommand
│   │   └── server/         # Server implementations
│   │       ├── server.go   # Combined server orchestrator
│   │       ├── http.go     # HTTP server
│   │       └── ssh.go      # SSH server
│   │
│   ├── features/           # Core feature implementations
│   │   ├── topten/        # Top Ten lists
│   │   │   ├── service.go
│   │   │   ├── printer.go
│   │   │   └── data.go
│   │   ├── images/        # Image processing
│   │   │   ├── processor.go
│   │   │   ├── signer.go
│   │   │   └── cache.go
│   │   └── auth/          # Authentication
│   │       ├── oauth.go
│   │       └── session.go
│   │
│   ├── web/               # Web-specific code
│   │   ├── handlers/      # HTTP handlers
│   │   │   ├── topten.go
│   │   │   ├── images.go
│   │   │   └── health.go
│   │   ├── middleware/    # HTTP middleware
│   │   │   ├── auth.go
│   │   │   └── logging.go
│   │   └── templates/     # HTML templates (gomponents)
│   │       └── layouts.go
│   │
│   └── shared/            # Shared utilities
│       ├── config.go      # Configuration management
│       ├── logger.go      # Logging setup
│       └── crypto.go      # Encryption helpers
│
├── assets/                # Embedded static assets
│   ├── data/
│   │   ├── topten.json.age
│   │   └── hostkey.age
│   └── embed.go          # go:embed directives
│
├── deploy/
│   ├── magic-container/
│   │   ├── Containerfile      # Minimal for bunny.net
│   │   ├── config.yaml        # Magic Container config
│   │   └── README.md          # Deployment instructions
│   ├── bootc/
│   │   ├── Containerfile      # Full system container
│   │   ├── systemd/
│   │   │   ├── prospero.service
│   │   │   └── prospero-ssh.service
│   │   ├── config.bu          # Butane config
│   │   └── README.md          # Bootc deployment guide
│   └── docker-compose.yml     # Local development
│
├── go.mod
├── go.sum
├── justfile
├── README.md
└── .env.example
```

## Architecture Overview

### Single Binary Design
- **One executable**: `prospero`
- **CLI mode**: `prospero [command]` for local use
- **Server mode**: `prospero serve` for edge deployment
- **Simplifies deployment** to Magic Containers

### Core Components

#### 1. Features (`/internal/features/`)
Self-contained business logic that can be used by both CLI and server:

- **topten**: David Letterman Top 10 lists
- **images**: Image processing, compression, and signed URLs
- **auth**: OAuth authentication and session management

#### 2. Application Layer (`/internal/app/`)
Entry points that coordinate features:

- **cli**: Command-line interface implementations
- **server**: HTTP and SSH server orchestration

#### 3. Web Layer (`/internal/web/`)
HTTP-specific components:

- **handlers**: HTTP request handlers
- **middleware**: Request processing (auth, logging, CORS)
- **templates**: HTML rendering with gomponents

#### 4. Shared Utilities (`/internal/shared/`)
Cross-cutting concerns used throughout the application.

## CLI Command Structure

```bash
# Root command shows help
$ prospero
A magical box of edge services and fun utilities

Commands:
  serve     Start the server (HTTP + SSH)
  topten    Display a random Top 10 list
  compress  Compress and optimize images
  sign-url  Generate a signed URL

# Server mode
$ prospero serve --http-port 8080 --ssh-port 2222

# CLI utilities
$ prospero topten
$ prospero compress input.jpg --output optimized.jpg --quality 85
$ prospero sign-url /images/cat.jpg --expires 3h
```

## Code Organization Examples

### Entry Point
```go
// cmd/prospero/main.go
package main

import (
    "github.com/urfave/cli/v2"
    "prospero/internal/app/cli"
)

func main() {
    app := cli.NewApp()
    app.Run(os.Args)
}
```

### CLI Root Command
```go
// internal/app/cli/root.go
package cli

func NewApp() *cli.App {
    return &cli.App{
        Name:  "prospero",
        Usage: "A magical box of edge services",
        Commands: []*cli.Command{
            serveCommand(),
            topTenCommand(),
            compressCommand(),
            signURLCommand(),
        },
    }
}
```

### Feature Service (Reusable)
```go
// internal/features/topten/service.go
package topten

// Shared between CLI and server
type Service struct {
    lists []List
}

func (s *Service) GetRandom() (*List, error) {
    // Implementation shared by CLI and HTTP handler
}
```

### Server Orchestrator
```go
// internal/app/server/server.go
package server

type Server struct {
    httpServer *http.Server
    sshServer  *ssh.Server
    features   *Features
}

func (s *Server) Start() error {
    // Start both HTTP and SSH servers
}
```

## Magic Containers Deployment

### Port Configuration
```yaml
Services:
  - HTTP/HTTPS: 8080 (main web interface)
  - SSH: 2222 (fun CLI access)
  - Health Check: 8080/health
```

### Environment Variables
```bash
# Secrets and configuration
TOP10_AGE_PASSPHRASE=xxx        # Decrypt data files
IMAGE_SIGNING_SECRET=xxx        # Sign image URLs  
OAUTH_CLIENT_ID=xxx            # OAuth provider
OAUTH_CLIENT_SECRET=xxx
S3_BUCKET=images               # Image storage
S3_ENDPOINT=xxx                # S3-compatible endpoint
SESSION_SECRET=xxx             # Cookie signing
```

### Container Features
- **Anycast IP**: Global access for both HTTP and SSH
- **AI Deployment**: Auto-provision in optimal regions  
- **Self-Healing**: Automatic failover and restart
- **Pay-per-Use**: Only pay for actual resource consumption

## Planned Features

### Fun Features (`/features/`)
- **Top Ten Lists**: David Letterman lists (existing)
- **Fortune Cookies**: Random fortunes and quotes
- **ASCII Art**: Text-to-ASCII conversion
- **Dad Jokes**: Curated joke API
- **Weather Haikus**: Poetic weather reports
- **This Day in History**: Historical facts

### Utility Features (`/features/`)
- **Image Processing**: Compression, format conversion, optimization
- **URL Signing**: Secure, expiring URLs for resources
- **OAuth Protection**: Secure access to admin features
- **QR Codes**: Generate QR codes for any content
- **URL Shortener**: Short links with analytics
- **Webhook Receiver**: Accept and process webhooks

### Service Endpoints

#### Public HTTP Endpoints
```
GET  /                        # Landing page
GET  /topten                  # Random Top 10 list (HTML)
GET  /api/topten              # Random Top 10 list (JSON)
POST /api/images/process      # Image processing
GET  /images/{id}             # Signed image URLs
GET  /health                  # Health check
```

#### Protected HTTP Endpoints
```
GET  /admin                   # Admin dashboard
GET  /api/admin/*            # Admin API endpoints
POST /api/admin/images       # Admin image management
```

#### SSH Access
```bash
# Fun SSH interface
ssh prospero.example.com -p 2222   # Get random Top 10 list
```

## Development Tools

### Justfile Commands
```makefile
# Build everything
build: build-cli build-server

# Build single binary
build:
    go build -o bin/prospero ./cmd/prospero

# Build for Magic Container (linux/amd64)
build-container:
    GOOS=linux GOARCH=amd64 go build -o bin/prospero-linux ./cmd/prospero

# Local development
run *args:
    ./bin/prospero {{args}}

serve *args:
    ./bin/prospero serve {{args}}

# Quick shortcuts
topten:
    ./bin/prospero topten

# Container operations
container-magic:
    podman build -f deploy/magic-container/Containerfile -t localhost/prospero:latest .

container-bootc:
    podman build -f deploy/bootc/Containerfile -t localhost/prospero-bootc:latest .

containers: container-magic container-bootc

container-run-magic:
    podman run -p 8080:8080 -p 2222:2222 localhost/prospero:latest

# Testing
test:
    go test ./...

check: test fmt lint

fmt:
    go fmt ./...

lint:
    golangci-lint run
```

## Migration Strategy

### Phase 1: Foundation
1. Create new `prospero` repository
2. Set up basic CLI structure with urfave/cli
3. Migrate Top Ten functionality as first feature
4. Implement `prospero topten` command

### Phase 2: Server Core  
1. Add `prospero serve` command
2. Migrate existing SSH server functionality
3. Add basic HTTP server with health check
4. Implement `/api/topten` endpoint

### Phase 3: Web Interface
1. Add HTML templates with gomponents
2. Create `/topten` web page
3. Add basic styling and layout
4. Implement landing page

### Phase 4: Image Processing
1. Add image processing feature
2. Implement `/api/images/process` endpoint
3. Add URL signing functionality
4. Set up S3-compatible storage integration

### Phase 5: Authentication
1. Add OAuth middleware
2. Implement session management
3. Create protected admin routes
4. Add user authentication flow

### Phase 6: Production Hardening
1. Add comprehensive logging
2. Implement rate limiting
3. Add monitoring/metrics
4. Security audit and testing
5. Magic Container deployment optimization

## Key Benefits

### 1. **Flexibility**
- Run as CLI tool locally for development/testing
- Deploy as full server to Magic Containers
- Same codebase, different execution modes

### 2. **Code Reuse**
- Features work in both CLI and server modes
- Shared business logic reduces duplication
- Consistent behavior across interfaces

### 3. **Maintainability**
- Clear separation of concerns
- Feature-oriented organization
- Easy to add new capabilities

### 4. **Extensibility**
- Simple pattern for adding new features
- Plugin-like architecture for fun features
- Easy A/B testing of new functionality

### 5. **Deployment Simplicity**
- Single binary simplifies containers
- No complex orchestration needed
- Perfect for Magic Containers edge deployment

### 6. **Development Experience**
- Fast iteration with local CLI
- Easy testing of server functionality
- Consistent tooling with justfile

## Testing Strategy

### Unit Tests
```go
// Example test structure
prospero/
├── internal/
│   ├── features/
│   │   ├── topten/
│   │   │   ├── service.go
│   │   │   └── service_test.go    # Feature tests
│   │   └── images/
│   │       ├── processor.go
│   │       └── processor_test.go  # Processing tests
│   └── web/
│       ├── handlers/
│       │   ├── topten.go
│       │   └── topten_test.go     # Handler tests
```

### Integration Tests
- Test CLI commands end-to-end
- Test HTTP endpoints with real requests
- Test SSH server connectivity
- Test with real S3 storage (Docker containers)

### Magic Container Testing
- Local Docker testing before deployment
- Staging deployment for validation
- Health check monitoring
- Performance benchmarking

This structure provides a solid foundation for building a comprehensive edge service platform while maintaining the fun, experimental nature that makes it a "magical box" of utilities.

## Container Deployment Strategies

### Dual Container Approach

Prospero supports two distinct container deployment strategies, each optimized for different use cases:

#### 1. Magic Container (Edge Deployment)

**File**: `deploy/magic-container/Containerfile`

```dockerfile
# Minimal container for bunny.net Magic Containers
FROM alpine:3.19 AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN apk add --no-cache go git
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o prospero ./cmd/prospero

# Minimal runtime
FROM scratch
COPY --from=builder /build/prospero /prospero
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
EXPOSE 8080 2222
ENTRYPOINT ["/prospero", "serve"]
```

**Optimized for**:
- Minimal size (10-20MB)
- Fast cold starts
- Edge deployment
- bunny.net Magic Containers

#### 2. Bootc Container (System Deployment)

**File**: `deploy/bootc/Containerfile`

```dockerfile
# System container for Fedora CoreOS
FROM quay.io/fedora/fedora-bootc:40 AS base

# Build stage
FROM golang:1.21 AS builder
WORKDIR /build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o prospero ./cmd/prospero

# Final bootc image
FROM base

# Install system dependencies
RUN dnf install -y \
    systemd \
    systemd-resolved \
    NetworkManager \
    podman \
    && dnf clean all

# Add prospero binary
COPY --from=builder /build/prospero /usr/local/bin/prospero

# Add systemd services
COPY deploy/bootc/systemd/*.service /etc/systemd/system/
RUN systemctl enable prospero.service

# Configure for bootc
LABEL ostree.bootable=true
```

**Optimized for**:
- Full system integration
- Systemd service management
- Infrastructure deployment
- Fedora CoreOS bootc

### Build Commands

```bash
# Using Podman (preferred)
podman build -f deploy/magic-container/Containerfile -t prospero:latest .
podman build -f deploy/bootc/Containerfile -t prospero-bootc:latest .

# Using Docker (CI/CD compatibility)
docker build -f deploy/magic-container/Containerfile -t prospero:latest .
docker build -f deploy/bootc/Containerfile -t prospero-bootc:latest .
```

## Bootc vs Normal Containers: Complete Comparison

### What is Bootc?

Bootc (short for "bootable containers") is a technology that allows OCI containers to become the root filesystem of a running system. Instead of installing a traditional OS and then running containers on top, bootc containers **become** the operating system.

### Technical Architecture Differences

#### Normal Containers
```
Host OS (RHEL, Ubuntu, etc.)
├── Container Runtime (Docker, Podman)
│   ├── Container 1 (prospero)
│   ├── Container 2 (database)
│   └── Container N (other services)
└── System Services (systemd, networking)
```

#### Bootc Containers
```
Bootc Container Image
├── Complete OS userspace (Fedora CoreOS)
├── Systemd (PID 1)
├── Application services
├── System configuration
└── All dependencies included
```

### Detailed Comparison

| Aspect | Normal Containers | Bootc Containers |
|--------|------------------|------------------|
| **Purpose** | Application packaging | Complete system images |
| **Runtime** | Runs on existing OS | Becomes the OS |
| **Size** | 10-100MB typical | 200MB-2GB typical |
| **Boot Process** | Container starts after OS | Container is the OS |
| **Updates** | Container image updates | Atomic OS updates |
| **Persistence** | Volumes/mounts | Layered filesystem |
| **Services** | Application only | Full systemd |
| **Security** | Container isolation | Full system control |

### Pros and Cons

#### Normal Containers

**Pros:**
✅ **Lightweight**: Minimal overhead, fast starts
✅ **Portable**: Runs anywhere containers are supported
✅ **Simple**: Easy to build, test, and deploy
✅ **Resource efficient**: Multiple containers share OS
✅ **Mature ecosystem**: Extensive tooling and platforms
✅ **Development friendly**: Quick iteration cycles
✅ **Cost effective**: Pay only for application resources

**Cons:**
❌ **Host dependency**: Relies on host OS management
❌ **Limited system control**: Can't modify kernel/systemd
❌ **Security boundaries**: Shared kernel with host
❌ **Configuration drift**: Host configuration can vary
❌ **Update complexity**: Host and container updates separate

#### Bootc Containers

**Pros:**
✅ **Complete control**: Full OS customization capability
✅ **Immutable infrastructure**: Atomic, reproducible deployments
✅ **Version everything**: OS + apps versioned together
✅ **Rollback capability**: Easy rollback to previous OS state
✅ **Security isolation**: Complete system boundary
✅ **Simplified operations**: One artifact for everything
✅ **Consistent environments**: Identical dev/test/prod

**Cons:**
❌ **Resource heavy**: Larger images, more memory usage
❌ **Complex builds**: Must include full OS dependencies
❌ **Slower iteration**: Longer build and boot times
❌ **Storage requirements**: More disk space needed
❌ **Update overhead**: Must update entire system image
❌ **Learning curve**: New concepts and tooling
❌ **Limited hosting**: Fewer platforms support bootc

### Use Case Guidelines

#### Choose Normal Containers When:

**Perfect for Prospero because:**
- ✅ **Edge deployment**: bunny.net Magic Containers optimized for this
- ✅ **Rapid iteration**: Quick updates and deployments
- ✅ **Resource constraints**: Minimal overhead for edge computing
- ✅ **Platform managed**: bunny.net handles the host OS
- ✅ **Microservices**: Single responsibility applications
- ✅ **Development speed**: Fast build-test-deploy cycles

**Other ideal scenarios:**
- Cloud platforms (AWS ECS, Google Cloud Run, Azure Container Instances)
- Kubernetes deployments
- CI/CD pipelines
- Development environments
- Stateless applications

#### Choose Bootc Containers When:

**Good for Prospero when:**
- ✅ **Infrastructure ownership**: Running on your own hardware/VMs
- ✅ **System integration**: Need custom kernel modules or system services
- ✅ **Compliance requirements**: Full control over OS stack
- ✅ **Edge computing**: IoT devices or edge nodes you manage
- ✅ **Immutable infrastructure**: Want GitOps-style OS management

**Other ideal scenarios:**
- IoT device firmware
- Edge computing nodes
- Embedded systems
- High-security environments
- Custom appliances
- Infrastructure as code deployments

### Prospero Deployment Strategy

#### Recommended Approach: Both Containers

**For Edge/Cloud Deployment** (Normal Container):
```bash
# Deploy to bunny.net Magic Containers
podman build -f deploy/magic-container/Containerfile -t prospero:edge .
# Push to registry and deploy via Magic Containers
```

**For Infrastructure Deployment** (Bootc Container):
```bash
# Build system image
podman build -f deploy/bootc/Containerfile -t prospero:bootc .
# Deploy to Fedora CoreOS nodes
bootc upgrade prospero:bootc
```

#### Decision Matrix

| Deployment Target | Container Type | Why |
|------------------|----------------|-----|
| bunny.net Magic Containers | Normal | Optimized platform, managed infrastructure |
| AWS/GCP/Azure | Normal | Platform handles OS, focus on application |
| Your own VMs/bare metal | Bootc | Full control, immutable infrastructure |
| IoT/Edge devices | Bootc | Appliance-like deployment, offline capable |
| Kubernetes | Normal | Platform abstraction, orchestration |
| Development laptop | Normal | Quick iteration, resource sharing |

### Future Considerations

#### Technology Evolution
- **Bootc is emerging**: Still maturing, but rapid development
- **Platform adoption**: More platforms will support bootc over time
- **Tooling improvements**: Better IDE integration and debugging tools coming
- **Ecosystem growth**: More base images and examples becoming available

#### Prospero Evolution
- Start with normal containers for immediate deployment
- Add bootc support for infrastructure use cases
- Maintain both as the ecosystem grows
- Consider hybrid approaches as tooling matures

The dual-container strategy ensures Prospero can adapt to different deployment needs while maximizing the benefits of each approach.