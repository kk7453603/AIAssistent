import { type KeyboardEvent, useEffect, useRef, useState } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { Loader2, Send } from "lucide-react";
import { useChatStore } from "../stores/chatStore";
import { useEscapeToHide, useQuickAskToggle } from "../hooks/useQuickAsk";

const QUICK_CONV_ID = "quick-ask";
const DEFAULT_MODEL = "llama3.1:8b";

export function QuickAskPage() {
  useQuickAskToggle();
  useEscapeToHide();

  const { messages, isStreaming, sendMessage, stopStreaming } = useChatStore();
  const [text, setText] = useState("");
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const handleSend = () => {
    const trimmed = text.trim();
    if (!trimmed || isStreaming) return;
    sendMessage(trimmed, QUICK_CONV_ID, DEFAULT_MODEL);
    setText("");
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div className="flex h-screen flex-col rounded-xl bg-white/90 dark:bg-gray-950/90 backdrop-blur-xl">
      {/* Drag region */}
      <div
        className="h-6 shrink-0 cursor-move rounded-t-xl"
        data-tauri-drag-region
      />

      {/* Messages */}
      <div className="flex-1 space-y-3 overflow-y-auto px-4">
        {messages.length === 0 && (
          <p className="py-8 text-center text-sm text-gray-500">
            Ask anything... (Esc to close)
          </p>
        )}
        {messages.map((msg, i) => (
          <div
            key={i}
            className={`text-sm ${
              msg.role === "user"
                ? "text-right text-blue-300"
                : "text-left text-gray-700 dark:text-gray-300"
            }`}
          >
            {msg.role === "assistant" ? (
              <div className="prose dark:prose-invert prose-sm max-w-none">
                <Markdown remarkPlugins={[remarkGfm]}>{msg.content}</Markdown>
              </div>
            ) : (
              <p>{msg.content}</p>
            )}
          </div>
        ))}
        {isStreaming && messages[messages.length - 1]?.role !== "assistant" && (
          <div className="flex items-center gap-2 text-sm text-gray-500">
            <Loader2 className="h-4 w-4 animate-spin" />
            Thinking...
          </div>
        )}
        <div ref={bottomRef} />
      </div>

      {/* Input */}
      <div className="shrink-0 border-t border-gray-200 dark:border-gray-800 p-3">
        <div className="flex items-end gap-2">
          <textarea
            value={text}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Quick question..."
            rows={1}
            className="flex-1 resize-none rounded-lg border border-gray-300 dark:border-gray-700 bg-gray-50/50 dark:bg-gray-800/50 px-3 py-2 text-sm text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 outline-none focus:border-blue-500"
          />
          {isStreaming ? (
            <button
              onClick={stopStreaming}
              className="flex h-9 w-9 items-center justify-center rounded-lg bg-red-600 text-white hover:bg-red-500"
            >
              <div className="h-3 w-3 rounded-sm bg-white" />
            </button>
          ) : (
            <button
              onClick={handleSend}
              disabled={!text.trim()}
              className="flex h-9 w-9 items-center justify-center rounded-lg bg-blue-600 text-white hover:bg-blue-500 disabled:opacity-40"
            >
              <Send className="h-4 w-4" />
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
