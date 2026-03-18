#!/bin/sh
set -eu

OPENWEBUI_URL="${OPENWEBUI_URL:-http://openwebui:8080}"
OPENWEBUI_ADMIN_EMAIL="${OPENWEBUI_ADMIN_EMAIL:-}"
OPENWEBUI_ADMIN_PASSWORD="${OPENWEBUI_ADMIN_PASSWORD:-}"
FUNCTION_ID="${FUNCTION_ID:-paa_rag_pipe}"
FUNCTION_NAME="${FUNCTION_NAME:-Personal AI Assistant}"
FUNCTION_TYPE="${FUNCTION_TYPE:-pipe}"
FUNCTION_FILE_PATH="${FUNCTION_FILE_PATH:-/function/paa_rag_pipe.py}"

if [ -z "$OPENWEBUI_ADMIN_EMAIL" ] || [ -z "$OPENWEBUI_ADMIN_PASSWORD" ]; then
  echo "OPENWEBUI_ADMIN_EMAIL and OPENWEBUI_ADMIN_PASSWORD are required"
  exit 1
fi

if [ ! -f "$FUNCTION_FILE_PATH" ]; then
  echo "Function file not found: $FUNCTION_FILE_PATH"
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
  --arg id "$FUNCTION_ID" \
  --arg name "$FUNCTION_NAME" \
  --arg type "$FUNCTION_TYPE" \
  --rawfile content "$FUNCTION_FILE_PATH" \
  '{id: $id, name: $name, type: $type, content: $content, meta: {description: ("RAG pipe: " + $name)}, is_active: true, is_global: true}')

status_code=$(curl -sS -o /tmp/fn_existing.json -w "%{http_code}" \
  -H "Authorization: Bearer $token" \
  "$OPENWEBUI_URL/api/v1/functions/id/$FUNCTION_ID")

endpoint="/api/v1/functions/create"
if [ "$status_code" = "200" ]; then
  endpoint="/api/v1/functions/id/$FUNCTION_ID/update"
fi

echo "Upserting function '$FUNCTION_ID' ($FUNCTION_TYPE) via $endpoint"
curl -fsS -X POST "$OPENWEBUI_URL$endpoint" \
  -H "Authorization: Bearer $token" \
  -H "Content-Type: application/json" \
  -d "$payload" >/tmp/fn_upsert.json

echo "Function bootstrap completed: $FUNCTION_NAME ($FUNCTION_ID)"
