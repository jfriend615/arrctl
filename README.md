# arrctl

Unified CLI for managing *arr services (Sonarr, Radarr, Overseerr, Tautulli).

## Features

- Single entry point for all *arr services
- POSIX compliant (works with sh, dash, bash)
- Config file or environment variable configuration
- Pipeable JSON output for scripting
- Minimal dependencies (curl, jq)

## Requirements

- POSIX shell (sh, dash, bash, etc.)
- curl
- jq

Optional:
- shellcheck (for development/testing)

## Installation

```sh
# Clone the repository
git clone https://github.com/jfriend615/arrctl.git
cd arrctl

# Make the main script executable (if not already)
chmod +x bin/arrctl

# Option 1: Add to PATH
export PATH="$PATH:$(pwd)/bin"

# Option 2: Symlink to a directory in your PATH
ln -s "$(pwd)/bin/arrctl" /usr/local/bin/arrctl
```

## Configuration

### Option 1: Config File (Recommended)

Create `~/.config/arrctl/config.json`:

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

A template is provided in `config/config.json`.

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
arrctl --config /path/to/config.json sonarr series list
```

## Usage

```sh
# Show help
arrctl --help

# Show version
arrctl --version

# Service commands (coming soon)
arrctl sonarr <command>
arrctl radarr <command>
arrctl overseerr <command>
arrctl tautulli <command>
```

## Project Structure

```
arrctl/
├── bin/
│   └── arrctl          # Main entry point
├── lib/
│   └── common.sh       # Shared utilities
├── config/
│   └── config.json     # Config template
├── .gitignore
└── README.md
```

## Development

### Testing

```sh
# Check shell syntax with shellcheck
shellcheck bin/arrctl lib/*.sh

# Test with dash (POSIX compliance)
dash bin/arrctl --help

# Validate config JSON
jq empty config/config.json
```

### Code Style

- POSIX /bin/sh compliant (no bashisms)
- shellcheck clean (no warnings or errors)
- DRY principles via common.sh
- Functions documented with usage comments

## License

MIT
