# GCPlane — Docker Compose Production Deployment

## Prerequisites

- Docker 24+
- Docker Compose v2 (`docker compose` not `docker-compose`)

## Quick Start

```bash
# 1. Copy files into your working directory
cp manifest.yaml.example manifest.yaml
cp ../../.env.example .env

# 2. Edit manifest.yaml with your GoClaw endpoint and resources
# 3. Fill in credentials in .env

# 4. Start
docker compose -f docker-compose.prod.yaml up -d
```

## Verify

```bash
docker compose -f docker-compose.prod.yaml ps
curl http://localhost:8480/healthz
curl http://localhost:8480/readyz
```

## Environment Variables

Copy `.env.example` from the repo root and fill in the required values:

| Variable | Required | Description |
|---|---|---|
| `GOCLAW_TOKEN` | Yes | GoClaw gateway token |
| `ANTHROPIC_API_KEY` | If used | Anthropic provider key |
| `OPENAI_API_KEY` | If used | OpenAI provider key |
| `GCPLANE_LOG_FORMAT` | No | `json` (default for prod) or `text` |
| `GCPLANE_WEBHOOK_URL` | No | Drift notification webhook |
| `GRAFANA_ADMIN_PASSWORD` | No | Grafana admin password (default: `admin`) |

Set `GCPLANE_LOG_FORMAT=json` in production to match the JSON log driver.

## With Monitoring (Prometheus + Grafana)

```bash
docker compose -f docker-compose.prod.yaml --profile monitoring up -d
```

- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin / `$GRAFANA_ADMIN_PASSWORD`)

Add Prometheus as a data source in Grafana: `http://prometheus:9090`

## Upgrade

```bash
docker compose -f docker-compose.prod.yaml pull
docker compose -f docker-compose.prod.yaml up -d
```

## Logs

```bash
docker compose -f docker-compose.prod.yaml logs -f gcplane
```

## Teardown

```bash
# Stop and remove containers (keeps volumes)
docker compose -f docker-compose.prod.yaml down

# Remove volumes too
docker compose -f docker-compose.prod.yaml down -v
```
