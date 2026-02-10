#!/bin/sh
set -eu

OPENWEBUI_URL="${OPENWEBUI_URL:-http://openwebui:8080}"
OPENWEBUI_ADMIN_EMAIL="${OPENWEBUI_ADMIN_EMAIL:-}"
OPENWEBUI_ADMIN_PASSWORD="${OPENWEBUI_ADMIN_PASSWORD:-}"
OPENWEBUI_TOOL_ID="${OPENWEBUI_TOOL_ID:-assistant_ingest_and_query}"
OPENWEBUI_TOOL_NAME="${OPENWEBUI_TOOL_NAME:-Assistant Ingest And Query}"
OPENWEBUI_TOOL_DESCRIPTION="${OPENWEBUI_TOOL_DESCRIPTION:-Uploads attached files and runs RAG query.}"
TOOL_FILE_PATH="${TOOL_FILE_PATH:-/tool/assistant_ingest_and_query.py}"

if [ -z "$OPENWEBUI_ADMIN_EMAIL" ] || [ -z "$OPENWEBUI_ADMIN_PASSWORD" ]; then
  echo "OPENWEBUI_ADMIN_EMAIL and OPENWEBUI_ADMIN_PASSWORD are required"
  exit 1
fi

if [ ! -f "$TOOL_FILE_PATH" ]; then
  echo "Tool file not found: $TOOL_FILE_PATH"
  exit 1
fi

echo "Waiting for OpenWebUI at $OPENWEBUI_URL ..."
ready=0
for i in $(seq 1 120); do
  if curl -fsS "$OPENWEBUI_URL/health" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 2
done

if [ "$ready" -ne 1 ]; then
  echo "OpenWebUI is not ready"
  exit 1
fi

token=""
for i in $(seq 1 60); do
  signin_payload=$(jq -n --arg email "$OPENWEBUI_ADMIN_EMAIL" --arg password "$OPENWEBUI_ADMIN_PASSWORD" '{email: $email, password: $password}')
  signin_response=$(curl -sS -X POST "$OPENWEBUI_URL/api/v1/auths/signin" \
    -H "Content-Type: application/json" \
    -d "$signin_payload" || true)
  token=$(printf "%s" "$signin_response" | jq -r '.token // empty')
  if [ -n "$token" ]; then
    break
  fi
  sleep 2
done

if [ -z "$token" ]; then
  echo "Failed to sign in OpenWebUI admin"
  exit 1
fi

payload=$(jq -n \
  --arg id "$OPENWEBUI_TOOL_ID" \
  --arg name "$OPENWEBUI_TOOL_NAME" \
  --arg description "$OPENWEBUI_TOOL_DESCRIPTION" \
  --rawfile content "$TOOL_FILE_PATH" \
  '{id: $id, name: $name, content: $content, meta: {description: $description}, access_control: null}')

status_code=$(curl -sS -o /tmp/tool_existing.json -w "%{http_code}" \
  -H "Authorization: Bearer $token" \
  "$OPENWEBUI_URL/api/v1/tools/id/$OPENWEBUI_TOOL_ID")

endpoint="/api/v1/tools/create"
if [ "$status_code" = "200" ]; then
  endpoint="/api/v1/tools/id/$OPENWEBUI_TOOL_ID/update"
fi

echo "Upserting tool '$OPENWEBUI_TOOL_ID' via $endpoint"
curl -fsS -X POST "$OPENWEBUI_URL$endpoint" \
  -H "Authorization: Bearer $token" \
  -H "Content-Type: application/json" \
  -d "$payload" >/tmp/tool_upsert.json

echo "Tool bootstrap completed"
