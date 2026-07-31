# arrctl

Unified CLI for managing *arr services (Sonarr, Radarr, Overseerr, Tautulli).

## Features

- Single entry point for all *arr services
- Config file or environment variable configuration
- Pipeable JSON output for scripting
- Minimal runtime dependencies

## Quick Start

### One-Line Install

```sh
curl -sSL https://raw.githubusercontent.com/jfriend615/arrctl/main/install.sh | sh
```

This will:
- Download the latest prebuilt `arrctl` binary from GitHub Releases
- Install it to `/usr/local/bin/arrctl` (or `BIN_DIR`)
- Generate a config template at `~/.config/arrctl/config.json`

### Custom Installation Options

```sh
# Install to a different bin directory
curl -sSL https://raw.githubusercontent.com/jfriend615/arrctl/main/install.sh | BIN_DIR=~/bin sh

# Install a specific release version
curl -sSL https://raw.githubusercontent.com/jfriend615/arrctl/main/install.sh | VERSION=v0.3.0 sh
```

### Manual Installation (from source)

```sh
git clone https://github.com/jfriend615/arrctl.git
cd arrctl
go build -o arrctl ./cmd/arrctl
install -m 755 arrctl /usr/local/bin/arrctl
```

### Updating

Run the installer again to fetch the latest release binary:

```sh
curl -sSL https://raw.githubusercontent.com/jfriend615/arrctl/main/install.sh | sh
```

Or pin a specific version:

```sh
curl -sSL https://raw.githubusercontent.com/jfriend615/arrctl/main/install.sh | VERSION=v0.3.0 sh
```

### Uninstalling

```sh
# Via Makefile (if installed from source)
make uninstall

# Or manually
rm /usr/local/bin/arrctl
rm -rf ~/.config/arrctl  # optional: remove config
```

## Requirements

The installed binary has no external runtime dependencies.

The one-line installer requires:

- curl
- tar
- shasum or sha256sum

Building from source or running tests requires Go 1.26+.

## Configuration

### Option 1: Config File (Recommended)

Create or edit `~/.config/arrctl/config.json`:

```json
{
  "sonarr": {
    "url": "http://localhost:8989",
    "api_key": "your-sonarr-api-key"
  },
  "radarr": {
    "url": "http://localhost:7878",
    "api_key": "your-radarr-api-key"
  },
  "overseerr": {
    "url": "http://localhost:5055",
    "api_key": "your-overseerr-api-key"
  },
  "tautulli": {
    "url": "http://localhost:8181",
    "api_key": "your-tautulli-api-key"
  }
}
```

The installer creates a template automatically. A sample is also in `config/config.json`.

### Option 2: Environment Variables

```sh
export SONARR_URL="http://localhost:8989"
export SONARR_API_KEY="your-api-key"

export RADARR_URL="http://localhost:7878"
export RADARR_API_KEY="your-api-key"

export OVERSEERR_URL="http://localhost:5055"
export OVERSEERR_API_KEY="your-api-key"

export TAUTULLI_URL="http://localhost:8181"
export TAUTULLI_API_KEY="your-api-key"
```

Environment variables take precedence over the config file.

### Option 3: Custom Config Path

```sh
# Via environment variable
export ARRCTL_CONFIG="/path/to/config.json"

# Or via command line
arrctl --config /path/to/config.json sonarr list
```

## Usage

```sh
# Show help
arrctl --help

# Show version
arrctl --version

# Install shell completion (bash/zsh)
arrctl completion --install

# Sonarr (TV shows)
arrctl sonarr list              # List all series
arrctl sonarr search "Breaking Bad"
arrctl sonarr add --id 12345 --search

# Radarr (Movies)
arrctl radarr list              # List all movies
arrctl radarr list --monitored  # Only monitored
arrctl radarr search "The Matrix"
arrctl radarr add --id 603 --search

# Overseerr (Requests)
arrctl overseerr pending        # View pending requests
arrctl overseerr approve 123
arrctl overseerr deny 456 --reason "Duplicate"

# Tautulli (Plex activity)
arrctl tautulli now             # Who's streaming right now
```

## Project Structure

```
arrctl/
├── cmd/arrctl/         # Go CLI entrypoint
├── internal/
│   ├── api/            # Unified HTTP clients for *arr + Tautulli
│   ├── commands/       # Cobra command handlers
│   ├── config/         # Config loader + env precedence
│   └── output/         # JSON/table rendering
├── go.mod
└── ...
```

## Development

### Setup

```sh
git clone https://github.com/jfriend615/arrctl.git
cd arrctl
```

### Testing

```sh
# Run Go tests
make test

# Run CLI, completion, and installer smoke tests
make test-shell
```

### Building

```sh
go build ./cmd/arrctl
```

### Updating Dependencies

```sh
go get -u ./...
go mod tidy
```

### Code Style

- Keep the CLI simple and user-facing behavior predictable.
- Prefer the standard library unless a dependency materially simplifies the code.
- Keep command logic covered by `go test ./...`.

## License

MIT
