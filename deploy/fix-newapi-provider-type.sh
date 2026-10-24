#!/usr/bin/env bash
# Fix the legacy PostgreSQL constraint that rejects provider_type=newapi.
#
# This is an operational one-shot helper. It executes the same DDL as
# backend/migrations/247_allow_newapi_upstream_provider_type.sql directly in
# the running Sub2API container, so it does not require rebuilding the image.
#
# Usage:
#   ./deploy/fix-newapi-provider-type.sh
#
# Override the container name when necessary:
#   SUB2API_CONTAINER_NAME=my-sub2api ./deploy/fix-newapi-provider-type.sh

set -euo pipefail

CONTAINER_NAME="${SUB2API_CONTAINER_NAME:-sub2api}"

if ! command -v docker >/dev/null 2>&1; then
    echo "错误：未找到 docker 命令。" >&2
    exit 1
fi

if ! docker inspect "$CONTAINER_NAME" >/dev/null 2>&1; then
    echo "错误：找不到容器 $CONTAINER_NAME。" >&2
    exit 1
fi

if [ "$(docker inspect -f '{{.State.Running}}' "$CONTAINER_NAME")" != "true" ]; then
    echo "错误：容器 $CONTAINER_NAME 当前未运行。" >&2
    exit 1
fi

echo "正在更新容器 $CONTAINER_NAME 中的 upstream_providers 约束..."

docker exec -i "$CONTAINER_NAME" sh -s <<'CONTAINER_SCRIPT'
set -eu

: "${DATABASE_HOST:?容器内未设置 DATABASE_HOST}"
: "${DATABASE_PORT:?容器内未设置 DATABASE_PORT}"
: "${DATABASE_USER:?容器内未设置 DATABASE_USER}"
: "${DATABASE_DBNAME:?容器内未设置 DATABASE_DBNAME}"
: "${DATABASE_PASSWORD:?容器内未设置 DATABASE_PASSWORD}"

PGPASSWORD="$DATABASE_PASSWORD" psql \
    -h "$DATABASE_HOST" \
    -p "$DATABASE_PORT" \
    -U "$DATABASE_USER" \
    -d "$DATABASE_DBNAME" \
    -X \
    -v ON_ERROR_STOP=1 <<'SQL'
BEGIN;

ALTER TABLE upstream_providers
    DROP CONSTRAINT IF EXISTS upstream_providers_provider_type_check;

ALTER TABLE upstream_providers
    ADD CONSTRAINT upstream_providers_provider_type_check
    CHECK (provider_type IN ('sub2api', 'newapi', 'codego'));

COMMIT;

SELECT conname, pg_get_constraintdef(oid)
FROM pg_constraint
WHERE conname = 'upstream_providers_provider_type_check';
SQL
CONTAINER_SCRIPT

echo "已完成。现在可以重新提交 NewAPI 上游供应商创建请求。"
