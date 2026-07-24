# Cosy-Template-Service

> A small Go microservice that fetches Cosy game-server templates from a GitHub repository and serves them over a versioned HTTP API.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8.svg?logo=go&logoColor=white)](./go.mod)
[![CI](https://github.com/magenta-mause/Cosy-Template-Service/actions/workflows/ci.yml/badge.svg)](https://github.com/magenta-mause/Cosy-Template-Service/actions/workflows/ci.yml)

Part of the [Cosy](https://github.com/magenta-mause/Cosy) project — a self-hostable game-server hosting and management platform.

## Overview

Cosy lets users deploy game servers from reusable **templates**. Those templates
(and the games they belong to) live as YAML files in the
[Cosy-Templates](https://github.com/magenta-mause/Cosy-Templates) repository so
they can be reviewed, versioned, and contributed to like any other source.

Cosy-Template-Service is the bridge between that repository and the rest of the
platform. It periodically pulls the template and game definitions straight from
GitHub, keeps them in memory, and exposes them through a simple read-only HTTP
API that the [Cosy-Backend](https://github.com/magenta-mause/Cosy-Backend) and
frontend consume. This keeps template content decoupled from application
deployments: publishing a new template is a commit to Cosy-Templates, not a
redeploy of the platform.

### Key features

- **GitHub-backed content** — reads `templates/*.yaml` and `games/*.yaml` from a
  configurable GitHub repository, branch, and path using the GitHub API.
- **Automatic refresh** — reloads templates and games in the background every
  3 minutes, so new commits appear without a restart.
- **Versioned API** — serves three template shapes (`v1`, `v2`, `v3`) so
  consumers can migrate independently, plus a games index endpoint.
- **In-memory & concurrency-safe** — snapshots are served under a read/write
  lock; a failed reload keeps the last good data.
- **Optional authentication** — runs unauthenticated (lower rate limits) or with
  a GitHub token for higher limits and private repositories.
- **Container-ready** — ships a distroless, non-root Docker image and Kubernetes
  manifests.

### Related repositories

| Repository | Purpose |
| --- | --- |
| [Cosy](https://github.com/magenta-mause/Cosy) | Main project / platform umbrella |
| [Cosy-Templates](https://github.com/magenta-mause/Cosy-Templates) | Source YAML templates and game definitions served by this service |
| [Cosy-Backend](https://github.com/magenta-mause/Cosy-Backend) | Core platform backend that consumes this API |
| [Cosy-Docs](https://github.com/magenta-mause/Cosy-Docs) | Project-wide documentation |

## Getting Started

### Prerequisites

- **Go 1.25.5+** (see [`go.mod`](./go.mod)) — required to build and run from source.
- **Docker** (optional) — to build and run the container image.
- **A GitHub personal access token** (optional) — only needed to raise API rate
  limits or read a private templates repository. The service runs without one.

### Installation

Clone the repository and build the binary:

```bash
git clone https://github.com/magenta-mause/Cosy-Template-Service.git
cd Cosy-Template-Service
go build ./...
```

Or build a container image:

```bash
docker build -t cosy-template-service .
```

### Configuration

Configuration is read at startup from a `config.yaml` file in the working
directory. Environment variables (loaded from a `.env` file if present) override
the corresponding file values — the GitHub token in particular is meant to be
supplied via the environment so it never lands in a config file.

**`config.yaml`** (checked into the repo with sensible defaults):

```yaml
github:
  owner: "Magenta-Mause"      # GitHub org/user that owns the templates repo
  repo: "Cosy-Templates"      # repository holding the template YAML files
  ref: "main"                 # branch, tag, or commit SHA to read from
  path: "templates"           # directory containing templates/*.yaml
  gamesPath: "games"          # directory containing games/*.yaml
port: 8080                     # HTTP port the service listens on
```

**Environment variables:**

| Variable | Required | Description |
| --- | --- | --- |
| `GITHUB_TOKEN` | No | GitHub token used to authenticate GitHub API calls. Without it the service uses an unauthenticated client with reduced rate limits. |

Copy [`.env.example`](./.env.example) to `.env` and fill in a token if you need
one:

```bash
cp .env.example .env
```

> The `github.token` config key is bound to `GITHUB_TOKEN`; keep tokens in the
> environment (or a Kubernetes secret) rather than in `config.yaml`. The `.env`
> file is git-ignored.

### Quick Start

From source:

```bash
go run ./cmd/templates-service
```

Or run the built binary / container:

```bash
./app                                   # binary produced by `go build`
docker run --rm -p 8080:8080 cosy-template-service
```

On startup the service loads templates and games from GitHub and listens on
`http://localhost:8080`. Verify it is up:

```bash
curl http://localhost:8080/templates
```

You should receive a JSON object of the form `{"templates": [ ... ]}`.

## API Documentation

All endpoints are `GET`, return JSON, and are read-only. CORS is enabled for all
origins (`GET`, `OPTIONS`).

| Method & Path | Description |
| --- | --- |
| `GET /templates` | All templates in **v1** shape (alias of `/v1/templates`). Uses `default` as the value key; resolves `game_id` to a numeric external id and omits fields carrying `{{var}}` placeholders. |
| `GET /v1/templates` | Same as `/templates`. |
| `GET /v2/templates` | All templates in **v2** shape. Like v1 but uses `default_value` as the value key. |
| `GET /v3/templates` | Raw templates with variables intact (`{{...}}` not resolved) and all newer fields (annotations, host mounts, string-capable resource limits / port mappings, `game_id` as-is). |
| `GET /v3/games` | The games index as a JSON array, sorted by slug. |

Responses are wrapped in a top-level key: template endpoints return
`{"templates": [...]}` and the games endpoint returns `{"games": [...]}`.

## Development

### Project structure

```
.
├── cmd/templates-service/   # main.go — entrypoint: loads config, wires
│                            #   config → github client → service → routes
├── internal/
│   ├── config/              # viper-based config loading (config.yaml + env)
│   ├── githubclient/        # GitHub API client: fetch templates & games
│   ├── models/              # Template (v1/v2/v3), Game, and scalar types
│   ├── server/              # gin route registration & HTTP handlers
│   └── templates/           # in-memory Service with background reload
├── k8s/                     # Kubernetes Deployment, Service, Ingress
├── config.yaml              # default runtime configuration
├── Dockerfile               # multi-stage build → distroless runtime image
└── .github/workflows/       # CI (build/test) and release (image publish)
```

### Available commands

```bash
go build ./...                    # build all packages
go run ./cmd/templates-service    # run the service locally
go test ./...                     # run the test suite
go vet ./...                      # static checks

docker build -t cosy-template-service .   # build the container image
```

### Development workflow

1. Fork/branch and make your changes.
2. Run `go build ./...` and `go test ./...` locally — CI runs the same commands
   on every push and pull request (see
   [`.github/workflows/ci.yml`](./.github/workflows/ci.yml)).
3. Run the service (`go run ./cmd/templates-service`) and hit the endpoints with
   `curl` to confirm behavior.
4. Open a pull request against `main`.

### Dependencies

Major direct dependencies (see [`go.mod`](./go.mod) for the full list):

- **[gin-gonic/gin](https://github.com/gin-gonic/gin)** — HTTP router / framework.
- **[google/go-github](https://github.com/google/go-github)** — GitHub API client.
- **[spf13/viper](https://github.com/spf13/viper)** — configuration (file + env).
- **[joho/godotenv](https://github.com/joho/godotenv)** — loads `.env` in development.
- **[golang.org/x/oauth2](https://pkg.go.dev/golang.org/x/oauth2)** — token auth for the GitHub client.
- **[gopkg.in/yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3)** — parses the template/game YAML.

### Deployment

The [`k8s/`](./k8s) directory contains manifests for a Kubernetes deployment
(`Deployment`, `Service`, and `Ingress`). The Deployment pulls the published
image from `ghcr.io/magenta-mause/cosy-template-service` and injects
`GITHUB_TOKEN` from a Kubernetes secret. Tagging a `vX.Y.Z` release builds and
pushes the image via
[`.github/workflows/release.yaml`](./.github/workflows/release.yaml).

## Documentation

Project-wide documentation lives in
[Cosy-Docs](https://github.com/magenta-mause/Cosy-Docs).

## Contributing

Contributions are welcome! Please open an issue to discuss substantial changes
first, and see the organization-wide guidelines in the
[magenta-mause/.github](https://github.com/magenta-mause/.github) repository.

- **Report a bug or request a feature:**
  [open an issue](https://github.com/magenta-mause/Cosy-Template-Service/issues).
- **Development setup:** see [Getting Started](#getting-started) and
  [Development](#development) above.

## License

Released under the [MIT License](./LICENSE).

## Contact / Support

For questions, bug reports, or feature requests, use the
[GitHub issue tracker](https://github.com/magenta-mause/Cosy-Template-Service/issues).
Broader project discussion belongs in the main
[Cosy](https://github.com/magenta-mause/Cosy) repository.
