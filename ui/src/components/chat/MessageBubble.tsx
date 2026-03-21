import { useState } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeHighlight from "rehype-highlight";
import { Check, ChevronDown, ChevronRight, Copy, Loader2 } from "lucide-react";
import type { ChatMessage, ToolStatusDelta } from "../../api/types";
import { parseThinkBlocks } from "../../utils/parseThinkBlocks";

interface Props {
  message: ChatMessage;
  toolStatus: ToolStatusDelta[];
  isStreaming: boolean;
}

/** Keep only the latest status per tool name. When streaming is done,
 *  promote any remaining "running" to "ok". */
function deduplicateToolStatus(
  statuses: ToolStatusDelta[],
  isStreaming: boolean,
): ToolStatusDelta[] {
  const map = new Map<string, ToolStatusDelta>();
  for (const ts of statuses) {
    map.set(ts.tool, ts);
  }
  if (!isStreaming) {
    for (const [key, ts] of map) {
      if (ts.status === "running") {
        map.set(key, { ...ts, status: "ok" });
      }
    }
  }
  return Array.from(map.values());
}

function ToolStatusLine({ tool }: { tool: ToolStatusDelta }) {
  return (
    <div className="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
      {tool.status === "running" && (
        <Loader2 className="h-3.5 w-3.5 animate-spin text-blue-400" />
      )}
      {tool.status === "ok" && (
        <Check className="h-3.5 w-3.5 text-green-400" />
      )}
      {tool.status === "error" && (
        <span className="text-red-400 text-xs">&#x2717;</span>
      )}
      <span>{tool.tool}</span>
    </div>
  );
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <button
      onClick={handleCopy}
      className="absolute right-2 top-2 rounded bg-gray-200 dark:bg-gray-700 p-1 text-gray-500 dark:text-gray-400 opacity-0 transition-opacity hover:text-gray-700 dark:hover:text-gray-200 group-hover:opacity-100"
    >
      {copied ? (
        <Check className="h-4 w-4" />
      ) : (
        <Copy className="h-4 w-4" />
      )}
    </button>
  );
}

function ThinkBlock({
  thinking,
  toolStatus,
  isStreaming,
}: {
  thinking: string;
  toolStatus: ToolStatusDelta[];
  isStreaming: boolean;
}) {
  const [open, setOpen] = useState(false);
  const hasContent = thinking || toolStatus.length > 0;
  if (!hasContent) return null;

  return (
    <div className="mb-3 rounded border border-gray-300 dark:border-gray-700 bg-gray-100/50 dark:bg-gray-800/50">
      <button
        onClick={() => setOpen(!open)}
        className="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
      >
        {open ? (
          <ChevronDown className="h-4 w-4" />
        ) : (
          <ChevronRight className="h-4 w-4" />
        )}
        <span>
          {isStreaming ? "Thinking..." : "Thought process"}
        </span>
        {isStreaming && (
          <Loader2 className="h-3.5 w-3.5 animate-spin text-blue-400" />
        )}
      </button>
      {open && (
        <div className="border-t border-gray-300 dark:border-gray-700 px-3 py-2">
          {toolStatus.length > 0 && (
            <div className="mb-2 space-y-1">
              {deduplicateToolStatus(toolStatus, isStreaming).map((ts, i) => (
                <ToolStatusLine key={i} tool={ts} />
              ))}
            </div>
          )}
          {thinking && (
            <pre className="whitespace-pre-wrap text-xs text-gray-500">
              {thinking}
            </pre>
          )}
        </div>
      )}
    </div>
  );
}

export function MessageBubble({ message, toolStatus, isStreaming }: Props) {
  const isUser = message.role === "user";

  if (isUser) {
    return (
      <div className="flex justify-end">
        <div className="max-w-[75%] rounded-2xl rounded-br-sm bg-blue-600 px-4 py-2.5 text-white">
          <p className="whitespace-pre-wrap">{message.content}</p>
        </div>
      </div>
    );
  }

  const { thinking, answer } = parseThinkBlocks(message.content);

  return (
    <div className="flex justify-start">
      <div className="max-w-[85%]">
        <ThinkBlock
          thinking={thinking}
          toolStatus={toolStatus}
          isStreaming={isStreaming}
        />
        <div className="prose dark:prose-invert prose-sm max-w-none">
          <Markdown
            remarkPlugins={[remarkGfm]}
            rehypePlugins={[rehypeHighlight]}
            components={{
              pre({ children, ...props }) {
                // eslint-disable-next-line @typescript-eslint/no-explicit-any
                const codeText =
                  typeof children === "string"
                    ? children
                    : (children as any)?.props?.children ?? "";
                return (
                  <div className="group relative">
                    <CopyButton text={String(codeText)} />
                    <pre {...props}>{children}</pre>
                  </div>
                );
              },
            }}
          >
            {answer}
          </Markdown>
        </div>
      </div>
    </div>
  );
}
