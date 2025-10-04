# Prospero

A magical box of edge services and fun utilities for deployment on bunny.net Magic Containers.

## Features

- **Single Binary**: One executable that works as both CLI tool and server
- **Top Ten Lists**: Dave's-style Top 10 lists
- **Shakespeare's Complete Works**: Access all 43 of Shakespeare's plays, poems, and sonnets
- **Multiple Interfaces**: CLI, HTTP REST API, and interactive SSH server
- **Edge Ready**: Designed for edge deployment on Magic Containers

## Quick Start

### Prerequisites

- Go 1.25+ installed
- `AGE_ENCRYPTION_PASSWORD` environment variable set
- `just` command runner (optional)

### Building

```bash
# Using just (recommended)
just build

# Or using go directly
go build -o bin/prospero ./cmd/prospero
```

### CLI Usage

```bash
# Show help
./bin/prospero --help

# Display a random Top 10 list
./bin/prospero topten

# Display in ASCII mode (good for terminals/SSH)
./bin/prospero topten --ascii

# Shakespeare commands
./bin/prospero shakespert works                    # List all works
./bin/prospero shakespert works --genre t          # Filter by tragedy
./bin/prospero shakespert work hamlet              # Show work details
./bin/prospero shakespert genres                   # List all genres
```

### Server Mode

```bash
# Start both HTTP (8080) and SSH (2222) servers
./bin/prospero serve

# Custom host and ports
./bin/prospero serve --host 0.0.0.0 --http-port 8080 --ssh-port 2222
```

### SSH Interface

```bash
# Show SSH help menu
ssh localhost -p 2222

# Get a random Top 10 list
ssh localhost -p 2222 topten

# Shakespeare commands
ssh localhost -p 2222 shakespert works             # List all works
ssh localhost -p 2222 shakespert work hamlet       # Show work details
ssh localhost -p 2222 shakespert genres            # List genres
```

### HTTP API

The server provides a REST API on port 8080:

```bash
# Health check
curl http://localhost:8080/health

# Top Ten Lists
curl http://localhost:8080/api/topten                    # JSON format
curl http://localhost:8080/api/topten?format=ascii      # Plain text

# Shakespeare API
curl http://localhost:8080/api/shakespert/works          # List all works (JSON)
curl http://localhost:8080/api/shakespert/works?format=text   # Plain text format
curl http://localhost:8080/api/shakespert/works?genre=t  # Filter by tragedy
curl http://localhost:8080/api/shakespert/works/hamlet   # Get work details
curl http://localhost:8080/api/shakespert/genres         # List all genres
```

### MCP Server

Prospero includes a Model Context Protocol (MCP) server that exposes prompts via stdio transport:

```bash
# Start the MCP server
./bin/prospero mcp

# Or using just
just mcp
```

#### Defining Prompts

Prompts are defined in TOML files in `assets/prompts/`. Example format:

```toml
name = "example_prompt"
description = "An example prompt that demonstrates the format"

[[arguments]]
name = "input"
description = "The input text to process"
required = true

[[arguments]]
name = "format"
description = "Output format (text or json)"
required = false
```

Each TOML file should define:
- `name` - Unique identifier for the prompt
- `description` - Human-readable description
- `arguments` - Array of argument definitions with name, description, and required flag

#### Claude Desktop Configuration

To use Prospero's MCP server with Claude Desktop, add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "prospero": {
      "command": "/path/to/bin/prospero",
      "args": ["mcp"]
    }
  }
}
```

## Development

### Development Commands

The `prospero` CLI includes a `dev` command for managing embedded data during development:

```bash
# Extract embedded data for local development
./bin/prospero dev extract              # Extract ALL data files
./bin/prospero dev extract topten       # Just topten.json (pretty-printed)
./bin/prospero dev extract hostkey      # Just SSH host key
./bin/prospero dev extract shakespert   # shakespert.sql + shakespert.db
./bin/prospero dev extract secrets      # All age-encrypted files

# Pack modified data back into embedded format
./bin/prospero dev pack shakespert      # Compress shakespert.db → assets/data/shakespert.sql.gz

# Rotate encryption keys (atomic operation)
export PREVIOUS_AGE_ENCRYPTION_PASSWORD="old_password"
export AGE_ENCRYPTION_PASSWORD="new_password"
./bin/prospero dev rotate-key           # Rotate all .age files
./bin/prospero dev rotate-key --dry-run # Test rotation without modifying files
```

### Just Commands

```bash
# Run tests
just test

# Clean build artifacts  
just clean

# Development shortcuts
just extract-all                         # Extract all embedded data
just pack-shakespert                     # Recompress shakespert after changes
just rotate-key                          # Rotate encryption keys

# Run with password prompt
just run-with-password

# Start server with password prompt
just serve-with-password

# Shakespeare commands
just shakespert-works                    # List all works
just shakespert-work hamlet              # Show work details
just shakespert-genres                   # List all genres

# API testing (requires server running)
just test-all-endpoints                  # Test all HTTP endpoints
just test-shakespert-api                 # Test just Shakespeare endpoints

# Development tools
just sqlc-generate                       # Generate SQL code from queries
```

### Development Workflow

1. **Extract data for development**:
   ```bash
   just extract-all
   # Or: ./bin/prospero dev extract
   ```

2. **Modify shakespert database**:
   ```bash
   sqlite3 shakespert.db
   # Make your changes...
   ```

3. **Repack the modified data**:
   ```bash
   just pack-shakespert
   # Or: ./bin/prospero dev pack shakespert
   ```

4. **Build and test**:
   ```bash
   just build
   just test
   ```

## Data Sources

- **Top Ten Lists**: Encrypted JSON data (Dave's archives)
- **Shakespeare Database**: Complete works database (~6MB compressed)
  - 43 works (plays, poems, sonnets)
  - 5 genres (Comedy, History, Poem, Sonnet, Tragedy) 
  - Full text searchable with metadata
- **SSH Host Key**: Encrypted Ed25519 key for SSH server

## Architecture

See `docs/prospero.md` for detailed architecture documentation.

## Environment Variables

- `AGE_ENCRYPTION_PASSWORD` - Password for decrypting data files (required)
- `PREVIOUS_AGE_ENCRYPTION_PASSWORD` - Previous password for key rotation (only needed when rotating keys)

## Genre Codes

- `c` - Comedy (e.g., Twelfth Night, As You Like It)
- `h` - History (e.g., Henry V, Richard III) 
- `p` - Poem (e.g., Venus and Adonis, The Rape of Lucrece)
- `s` - Sonnet (Sonnets 1-154)
- `t` - Tragedy (e.g., Hamlet, Macbeth, King Lear)