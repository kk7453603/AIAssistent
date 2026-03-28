import { useEffect, useRef, useState } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeHighlight from "rehype-highlight";
import { Check, ChevronDown, ChevronRight, Copy, Loader2, Sparkles } from "lucide-react";
import type { ChatMessage, ToolStatusDelta, OrchestrationStepEvent } from "../../api/types";
import { parseThinkBlocks } from "../../utils/parseThinkBlocks";
import { OrchestrationStepper } from "./OrchestrationStepper";

interface Props {
  message: ChatMessage;
  toolStatus: ToolStatusDelta[];
  orchSteps: OrchestrationStepEvent[];
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

function useThinkingTimer(isThinking: boolean): number {
  const [seconds, setSeconds] = useState(0);
  const startRef = useRef<number | null>(null);

  useEffect(() => {
    if (isThinking) {
      startRef.current = Date.now();
      const interval = setInterval(() => {
        setSeconds(Math.floor((Date.now() - (startRef.current ?? Date.now())) / 1000));
      }, 1000);
      return () => clearInterval(interval);
    } else if (startRef.current) {
      setSeconds(Math.floor((Date.now() - startRef.current) / 1000));
    }
  }, [isThinking]);

  return seconds;
}

function formatThinkingTime(seconds: number): string {
  if (seconds < 1) return "";
  if (seconds < 60) return `${seconds} sec`;
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return `${mins}m ${secs}s`;
}

function ThinkBlock({
  thinking,
  toolStatus,
  orchSteps,
  isStreaming,
}: {
  thinking: string;
  toolStatus: ToolStatusDelta[];
  orchSteps: OrchestrationStepEvent[];
  isStreaming: boolean;
}) {
  const [open, setOpen] = useState(false);
  const hasContent = thinking || toolStatus.length > 0 || orchSteps.length > 0;
  const contentRef = useRef<HTMLDivElement>(null);

  const isActivelyThinking = isStreaming && !!thinking;
  const thinkingSeconds = useThinkingTimer(isActivelyThinking);

  if (!hasContent) return null;

  const effectiveOpen = open || (orchSteps.length > 0 && isStreaming);

  const isOrchestration = orchSteps.length > 0;

  // Label logic
  let label: string;
  if (isOrchestration) {
    label = isStreaming ? "Multi-Agent Orchestration..." : "Multi-Agent Orchestration";
  } else if (isStreaming && thinking) {
    label = "Thinking...";
  } else {
    const time = formatThinkingTime(thinkingSeconds);
    label = time ? `Thought for ${time}` : "Thought process";
  }

  return (
    <div className="mb-3">
      <button
        onClick={() => setOpen(!open)}
        className="group flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm transition-colors hover:bg-amber-50 dark:hover:bg-amber-950/20"
      >
        {/* Sparkles icon with amber color */}
        <Sparkles
          className={`h-4 w-4 ${
            isActivelyThinking
              ? "text-amber-500 animate-pulse"
              : "text-amber-400 dark:text-amber-500"
          }`}
        />

        <span className="text-gray-600 dark:text-gray-400 group-hover:text-gray-800 dark:group-hover:text-gray-200">
          {label}
        </span>

        {isActivelyThinking && (
          <span className="h-1.5 w-1.5 rounded-full bg-amber-500 animate-pulse" />
        )}

        {effectiveOpen ? (
          <ChevronDown className="h-3.5 w-3.5 text-gray-400" />
        ) : (
          <ChevronRight className="h-3.5 w-3.5 text-gray-400" />
        )}
      </button>

      {/* Animated content */}
      <div
        ref={contentRef}
        className={`overflow-hidden transition-all duration-300 ease-in-out ${
          effectiveOpen ? "max-h-[2000px] opacity-100" : "max-h-0 opacity-0"
        }`}
      >
        <div className="mt-1 ml-3 border-l-2 border-amber-200 dark:border-amber-800/50 pl-4 py-2">
          {/* Orchestration stepper */}
          {orchSteps.length > 0 && (
            <div className="mb-3">
              <OrchestrationStepper steps={orchSteps} />
            </div>
          )}

          {/* Tool status */}
          {toolStatus.length > 0 && (
            <div className="mb-3 space-y-1">
              {deduplicateToolStatus(toolStatus, isStreaming).map((ts, i) => (
                <ToolStatusLine key={i} tool={ts} />
              ))}
            </div>
          )}

          {/* Thinking text */}
          {thinking && (
            <div className="text-[13px] leading-relaxed text-gray-500 dark:text-gray-400 italic whitespace-pre-wrap">
              {thinking}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export function MessageBubble({ message, toolStatus, orchSteps, isStreaming }: Props) {
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
          orchSteps={orchSteps}
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
