FROM node:22-slim

# Install python3, pip, and bash (bash is available in slim but python3 is not).
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        python3 \
        python3-pip \
        bash \
    && rm -rf /var/lib/apt/lists/*

# Install supergateway globally (used as CLI entrypoint).
RUN npm install -g supergateway@latest && npm cache clean --force

# Create non-root user for sandboxed code execution.
RUN useradd -m -s /bin/bash runner

# Set up application directory with MCP SDK as local dependency.
WORKDIR /app
COPY server.mjs ./server.mjs
RUN npm init -y && npm install @modelcontextprotocol/sdk@latest

ENV PORT=3001
ENV MCP_PATH=/mcp

EXPOSE 3001

# Run as non-root user.
USER runner

# supergateway invokes server.mjs via node in stdio mode, then bridges it to
# Streamable HTTP so the PAA Go MCP client can connect at http://mcp-code-runner:3001/mcp
CMD supergateway \
    --stdio "node /app/server.mjs" \
    --outputTransport streamableHttp \
    --port "${PORT}" \
    --streamableHttpPath "${MCP_PATH}" \
    --cors \
    --healthEndpoint /healthz \
    --logLevel info
