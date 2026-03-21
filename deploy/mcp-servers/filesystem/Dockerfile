FROM node:22-slim

# Install @modelcontextprotocol/server-filesystem and supergateway globally.
# supergateway bridges the stdio-based MCP server to Streamable HTTP so the
# PAA Go MCP client can connect via HTTP.
RUN npm install -g \
    @modelcontextprotocol/server-filesystem@latest \
    supergateway@latest \
    && npm cache clean --force

# Allowed directories are passed as positional arguments to the filesystem
# server command.  The entrypoint script reads ALLOWED_DIRS (space-separated)
# and falls back to /allowed/vaults when it is not set.
ENV ALLOWED_DIRS="/allowed/vaults"
ENV PORT=3000
ENV MCP_PATH=/mcp

EXPOSE 3000

# supergateway wraps the stdio server and exposes Streamable HTTP on $PORT at $MCP_PATH.
# --cors allows the PAA API container to call in without origin restrictions.
# --healthEndpoint /healthz makes it easy to add a health-check later.
CMD supergateway \
    --stdio "npx -y @modelcontextprotocol/server-filesystem ${ALLOWED_DIRS}" \
    --outputTransport streamableHttp \
    --port "${PORT}" \
    --streamableHttpPath "${MCP_PATH}" \
    --cors \
    --healthEndpoint /healthz \
    --logLevel info
