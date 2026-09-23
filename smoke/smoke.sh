#!/usr/bin/env bash
# Ручной smoke-тест sshocks через docker.
# Предпосылки: docker, bash, curl или netcat, Go 1.27.1.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

SSH_PORT="2222"
SSOCKS="127.0.0.1:1080"
TARGET="127.0.0.1:8080"

# 1. Локальный sshd на 127.0.0.1:$SSH_PORT + эхо-сервер на 8080 внутри контейнера.
docker compose -f smoke/docker-compose.yml up -d
sleep 3

# 2. Сборка и запуск sshocks.
go build -o /tmp/sshocks_smoke ./cmd/sshocks
/tmp/sshocks_smoke --config smoke/config.yaml &
SHOCKS=$!
sleep 2

# 3. Тест через туннель: curl → SOCKS5 → sshd → эхо-сервер 8080.
echo "--- curl через туннель ---"
if command -v curl >/dev/null 2>&1; then
      curl -s --socks5 "$SSOCKS" "http://$TARGET/hello" || true
  fi

# 4. Graceful shutdown через SIGINT → drain → выход 0.
echo "--- SIGINT → drain ---"
kill -INT "$SHOCKS"
wait "$SHOCKS"
echo "exit=$? (ожидаемо 0)"

# 5. Лог последних событий.
echo "--- last 20 log lines ---"
tail -20 sshocks.log || true

# Очистка.
docker compose -f smoke/docker-compose.yml down || true
rm -f /tmp/sshocks_smoke
