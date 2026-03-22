# game-master

Project foundation for the Game Master Go application.

## Prerequisites

- Go 1.24+
- Docker + Docker Compose
- [Task](https://taskfile.dev/)

## Repository layout

- `cmd/tui` – TUI entrypoint
- `cmd/server` – server entrypoint
- `internal/*` – core application packages
- `pkg/api` – exported API types
- `migrations/` – goose SQL migrations

## Quick start

1. Start local dependencies:
   ```bash
   docker compose up -d
   ```
2. Optionally copy the example configuration and adjust values:
   ```bash
   cp config/config.example.yaml config.yaml
   ```
3. Run database migrations:
   ```bash
   task migrate
   ```
4. Generate sqlc code:
   ```bash
   task generate
   ```
5. Run tests:
   ```bash
   task test
   ```
6. Build the binaries:
   ```bash
   task build
   ```

## Configuration

Configuration is loaded with koanf from a YAML file and then overridden by environment variables prefixed with `GM_`.

Example environment variables:

- `GM_DB_URL`
- `GM_LLM_PROVIDER`
- `GM_LLM_OLLAMA_ENDPOINT`
- `GM_LLM_OLLAMA_MODEL`
