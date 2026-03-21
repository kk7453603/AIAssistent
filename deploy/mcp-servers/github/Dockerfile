FROM node:22-slim

# Install @modelcontextprotocol/server-github and supergateway globally.
# supergateway bridges the stdio-based MCP server to Streamable HTTP so the
# PAA Go MCP client can connect via HTTP.
RUN npm install -g \
    @modelcontextprotocol/server-github@latest \
    supergateway@latest \
    && npm cache clean --force

# GITHUB_PERSONAL_ACCESS_TOKEN is read directly by @modelcontextprotocol/server-github
# and must be injected at runtime via the environment (docker-compose or .env).
ENV PORT=3002
ENV MCP_PATH=/mcp

EXPOSE 3002

# supergateway wraps the stdio server and exposes Streamable HTTP on $PORT at $MCP_PATH.
# --cors allows the PAA API container to call in without origin restrictions.
# --healthEndpoint /healthz makes it easy to add a health-check later.
CMD supergateway \
    --stdio "npx -y @modelcontextprotocol/server-github" \
    --outputTransport streamableHttp \
    --port "${PORT}" \
    --streamableHttpPath "${MCP_PATH}" \
    --cors \
    --healthEndpoint /healthz \
    --logLevel info
