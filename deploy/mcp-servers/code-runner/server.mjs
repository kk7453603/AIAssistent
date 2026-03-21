#!/usr/bin/env node
/**
 * Code Runner MCP Server (stdio mode)
 *
 * Provides two tools:
 *   - execute_python  — runs arbitrary Python 3 code via `python3 -c`
 *   - execute_bash    — runs arbitrary Bash commands via `bash -c`
 *
 * Output (stdout + stderr combined) is truncated to MAX_OUTPUT_BYTES to keep
 * LLM context usage reasonable.  The server runs in stdio mode and is wrapped
 * by supergateway to expose Streamable HTTP.
 */

import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
} from "@modelcontextprotocol/sdk/types.js";
import { exec } from "node:child_process";
import { writeFileSync, unlinkSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { randomBytes } from "node:crypto";

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const MAX_OUTPUT_BYTES = 10 * 1024; // 10 KB
const DEFAULT_TIMEOUT_SECONDS = 30;

// ---------------------------------------------------------------------------
// Helper: run a shell command with timeout, return combined output string
// ---------------------------------------------------------------------------

/**
 * @param {string} shell   - "python3" | "bash"
 * @param {string} code    - code / command string to execute
 * @param {number} timeout - timeout in seconds
 * @returns {Promise<{output: string, exitCode: number}>}
 */
function runCode(shell, code, timeout) {
  return new Promise((resolve) => {
    const timeoutMs = Math.max(1, Math.min(timeout, 120)) * 1000;

    // Write code to a temp file to avoid shell escaping issues with multiline code.
    const ext = shell === "python3" ? ".py" : ".sh";
    const tmpFile = join(tmpdir(), `mcp-${randomBytes(8).toString("hex")}${ext}`);
    writeFileSync(tmpFile, code, "utf-8");
    const cmd = `${shell} ${tmpFile}`;

    const child = exec(cmd, {
      timeout: timeoutMs,
      maxBuffer: MAX_OUTPUT_BYTES * 4, // node buffer; we truncate ourselves
      env: {
        ...process.env,
        // Prevent python from writing .pyc files in the container
        PYTHONDONTWRITEBYTECODE: "1",
        // Unbuffered python output
        PYTHONUNBUFFERED: "1",
      },
    });

    let stdout = "";
    let stderr = "";

    child.stdout?.on("data", (chunk) => {
      stdout += chunk;
    });
    child.stderr?.on("data", (chunk) => {
      stderr += chunk;
    });

    child.on("close", (code, signal) => {
      // Clean up temp file.
      try { unlinkSync(tmpFile); } catch {}

      let combined = "";
      if (stdout) combined += stdout;
      if (stderr) {
        if (combined) combined += "\n--- stderr ---\n";
        combined += stderr;
      }

      if (!combined && signal) {
        combined = `Process killed by signal: ${signal}`;
      }

      // Truncate to MAX_OUTPUT_BYTES
      const encoder = new TextEncoder();
      const bytes = encoder.encode(combined);
      if (bytes.length > MAX_OUTPUT_BYTES) {
        const truncated = new TextDecoder().decode(
          bytes.slice(0, MAX_OUTPUT_BYTES)
        );
        combined =
          truncated +
          `\n\n[...output truncated at ${MAX_OUTPUT_BYTES} bytes...]`;
      }

      resolve({
        output: combined || "(no output)",
        exitCode: code ?? -1,
      });
    });

    child.on("error", (err) => {
      resolve({
        output: `Execution error: ${err.message}`,
        exitCode: -1,
      });
    });
  });
}

// ---------------------------------------------------------------------------
// Tool definitions
// ---------------------------------------------------------------------------

const TOOLS = [
  {
    name: "execute_python",
    description:
      "Execute arbitrary Python 3 code and return stdout + stderr. " +
      "Output is limited to 10 KB. Maximum timeout is 120 seconds.",
    inputSchema: {
      type: "object",
      properties: {
        code: {
          type: "string",
          description: "Python 3 code to execute.",
        },
        timeout_seconds: {
          type: "number",
          description:
            "Execution timeout in seconds (1–120). Defaults to 30.",
          default: DEFAULT_TIMEOUT_SECONDS,
        },
      },
      required: ["code"],
    },
  },
  {
    name: "execute_bash",
    description:
      "Execute an arbitrary Bash command / script and return stdout + stderr. " +
      "Output is limited to 10 KB. Maximum timeout is 120 seconds.",
    inputSchema: {
      type: "object",
      properties: {
        command: {
          type: "string",
          description: "Bash command or script to execute.",
        },
        timeout_seconds: {
          type: "number",
          description:
            "Execution timeout in seconds (1–120). Defaults to 30.",
          default: DEFAULT_TIMEOUT_SECONDS,
        },
      },
      required: ["command"],
    },
  },
];

// ---------------------------------------------------------------------------
// MCP Server
// ---------------------------------------------------------------------------

const server = new Server(
  { name: "code-runner", version: "1.0.0" },
  { capabilities: { tools: {} } }
);

// List tools
server.setRequestHandler(ListToolsRequestSchema, async () => {
  return { tools: TOOLS };
});

// Call tool
server.setRequestHandler(CallToolRequestSchema, async (request) => {
  const { name, arguments: args } = request.params;

  if (name === "execute_python") {
    const code = String(args?.code ?? "");
    const timeout = Number(args?.timeout_seconds ?? DEFAULT_TIMEOUT_SECONDS);

    if (!code.trim()) {
      return {
        content: [{ type: "text", text: "Error: 'code' must not be empty." }],
        isError: true,
      };
    }

    const { output, exitCode } = await runCode("python3", code, timeout);
    const header = exitCode === 0 ? "" : `[exit code: ${exitCode}]\n`;

    return {
      content: [{ type: "text", text: header + output }],
      isError: exitCode !== 0,
    };
  }

  if (name === "execute_bash") {
    const command = String(args?.command ?? "");
    const timeout = Number(args?.timeout_seconds ?? DEFAULT_TIMEOUT_SECONDS);

    if (!command.trim()) {
      return {
        content: [
          { type: "text", text: "Error: 'command' must not be empty." },
        ],
        isError: true,
      };
    }

    const { output, exitCode } = await runCode("bash", command, timeout);
    const header = exitCode === 0 ? "" : `[exit code: ${exitCode}]\n`;

    return {
      content: [{ type: "text", text: header + output }],
      isError: exitCode !== 0,
    };
  }

  return {
    content: [{ type: "text", text: `Unknown tool: ${name}` }],
    isError: true,
  };
});

// ---------------------------------------------------------------------------
// Start
// ---------------------------------------------------------------------------

const transport = new StdioServerTransport();
await server.connect(transport);
// Intentionally silent on stderr to avoid polluting the supergateway pipe.
