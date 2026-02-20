# arrctl

Unified CLI for managing *arr services (Sonarr, Radarr, Overseerr, Tautulli).

## Features

- Single entry point for all *arr services
- POSIX compliant (works with sh, dash, bash)
- Config file or environment variable configuration
- Pipeable JSON output for scripting
- Minimal dependencies (curl, jq)

## Quick Start

### One-Line Install

```sh
curl -sSL https://raw.githubusercontent.com/jfriend615/arrctl/main/install.sh | sh
```

This will:
- Clone arrctl to `~/.arrctl`
- Create a symlink in `/usr/local/bin`
- Generate a config template at `~/.config/arrctl/config.json`

### Custom Installation Paths

```sh
# Install to a different directory
INSTALL_DIR=~/tools/arrctl BIN_DIR=~/bin curl -sSL ... | sh

# Or set variables before running
export INSTALL_DIR="$HOME/tools/arrctl"
export BIN_DIR="$HOME/bin"
curl -sSL https://raw.githubusercontent.com/jfriend615/arrctl/main/install.sh | sh
```

### Manual Installation

```sh
# Clone the repository
git clone https://github.com/jfriend615/arrctl.git ~/.arrctl

# Create symlink (may need sudo)
ln -s ~/.arrctl/bin/arrctl /usr/local/bin/arrctl

# Or add to PATH in your shell profile
echo 'export PATH="$PATH:$HOME/.arrctl/bin"' >> ~/.bashrc
```

### Updating

Run the installer again - it will pull the latest changes:

```sh
curl -sSL https://raw.githubusercontent.com/jfriend615/arrctl/main/install.sh | sh
```

Or manually:

```sh
cd ~/.arrctl && git pull
```

### Uninstalling

```sh
# Via Makefile (if installed from source)
make uninstall

# Or manually
rm /usr/local/bin/arrctl
rm -rf ~/.arrctl
rm -rf ~/.config/arrctl  # optional: remove config
```

## Requirements

- POSIX shell (sh, dash, bash, etc.)
- curl
- jq
- git (for installation)

Optional:
- shellcheck (for development/linting)

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
arrctl overseerr approve --id 123
arrctl overseerr deny --id 456 --reason "Duplicate"

# Tautulli (Plex activity)
arrctl tautulli now             # Who's streaming right now
```

## Project Structure

```
arrctl/
├── bin/
│   └── arrctl          # Main entry point
├── lib/
│   ├── common.sh       # Shared utilities
│   ├── sonarr.sh       # Sonarr commands
│   ├── radarr.sh       # Radarr commands
│   ├── overseerr.sh    # Overseerr commands
│   └── tautulli.sh     # Tautulli commands
├── completions/
│   ├── arrctl.bash     # Bash completion
│   ├── _arrctl         # Zsh completion
│   └── install.sh      # Completion installer
├── config/
│   └── config.json     # Config template
├── test/
│   └── smoke.sh        # Smoke tests
├── install.sh          # Installer script
├── Makefile            # Development tasks
├── .gitignore
└── README.md
```

## Development

### Setup

```sh
git clone https://github.com/jfriend615/arrctl.git
cd arrctl
```

### Testing

```sh
# Run smoke tests
make test

# Or directly
./test/smoke.sh
```

### Linting

```sh
# Check all scripts with shellcheck
make lint

# Or directly
shellcheck bin/arrctl lib/*.sh install.sh
```

### POSIX Compliance

```sh
# Test with dash (stricter than bash)
dash bin/arrctl --help
```

### Code Style

- POSIX /bin/sh compliant (no bashisms)
- shellcheck clean (no warnings or errors)
- DRY principles via common.sh
- Functions documented with usage comments

## License

MIT
