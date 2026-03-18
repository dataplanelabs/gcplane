#!/usr/bin/env bash
# Manage GoClaw docker compose instance
set -euo pipefail

GOCLAW_DIR="${GOCLAW_DIR:-../../nextlevelbuilder/goclaw}"
COMPOSE="docker compose -f docker-compose.yml -f docker-compose.postgres.yml -f docker-compose.selfservice.yml"

cd "$GOCLAW_DIR"

case "${1:-help}" in
  up)
    $COMPOSE up -d --build
    ;;
  down)
    $COMPOSE down
    ;;
  reset)
    $COMPOSE down -v
    $COMPOSE build --no-cache
    $COMPOSE up -d
    ;;
  logs)
    $COMPOSE logs -f goclaw
    ;;
  *)
    echo "Usage: goclaw.sh {up|down|reset|logs}"
    exit 1
    ;;
esac
