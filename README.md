<p align="center">
  <img src="assets/template.png" alt="Chi Template preview" width="150" />
</p>

<h1 align="center">Chi Template</h1>

<p align="center">
  Production-first Go template for Chi APIs.
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-green" alt="License: MIT" /></a>
  <img src="https://img.shields.io/badge/platform-linux%20%7C%20macos%20%7C%20windows-blue" alt="Platforms" />
  <img src="https://img.shields.io/badge/interface-TUI%20%2B%20CLI-7c3aed" alt="Interface" />
</p>

Chi Template helps you bootstrap clean backend services fast, with sensible defaults, optional infrastructure blocks, and both interactive and scripted workflows.

Built with Bubble Tea, Lip Gloss and Bubbles.

## Features

- Interactive TUI flow for quick setup
- CLI mode for automation and CI pipelines
- Standard Go project layout with clear boundaries
- Optional blocks: postgres, redis, observability, docker
- Generated projects include Makefile, env template, watch config and starter README
- Production-focused defaults: config, lifecycle, health probes and quality commands

## Installation

### Go install

```bash
go install github.com/mexirica/chi-template@latest
```

### Build from source

```bash
git clone https://github.com/mexirica/chi-template.git
cd chi-template
go build -o chi-template ./cmd
```

## Usage

### Interactive mode

```bash
go run ./cmd
```

### CLI mode

```bash
go run ./cmd create --name my-service --module github.com/you/my-service --output ../my-service --postgres --docker
```

### Block list mode

```bash
go run ./cmd create --name my-service --blocks postgres,redis,observability,docker
```

## Commands

| Command | Description |
|---|---|
| chi-template | Open interactive TUI |
| chi-template create [flags] | Generate project using flags |

## Create Flags

| Flag | Description |
|---|---|
| --name | Project name (required) |
| --module | Go module path |
| --output | Output directory |
| --blocks | Comma-separated blocks |
| --postgres | Enable postgres block |
| --redis | Enable redis block |
| --observability | Enable observability block |
| --docker | Enable docker block |

## Generated Layout

Base structure:

- cmd/api
- internal/app
- internal/server
- internal/config
- internal/logging
- internal/helpers
- internal/validation

Optional additions:

- postgres -> internal/database
- redis -> internal/cache and internal/middleware
- observability -> internal/o11y and local telemetry stack files
- docker -> deploy Dockerfile and Compose stack

## Local Development

```bash
make run
make build
make test
make tidy
```

## Philosophy

- Minimal by default
- Explicit over magical
- Fast bootstrap, easy evolution
- Production concerns included from day one
