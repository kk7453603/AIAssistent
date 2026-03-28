import { useEffect, useRef } from "react";
import type { ChatMessage, ToolStatusDelta, OrchestrationStepEvent } from "../../api/types";
import { MessageBubble } from "./MessageBubble";

interface Props {
  messages: ChatMessage[];
  toolStatus: ToolStatusDelta[];
  orchSteps: OrchestrationStepEvent[];
  isStreaming: boolean;
}

export function MessageList({ messages, toolStatus, orchSteps, isStreaming }: Props) {
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, toolStatus]);

  if (messages.length === 0) {
    return (
      <div className="flex flex-1 items-center justify-center text-gray-400 dark:text-gray-500">
        <p>Start a conversation...</p>
      </div>
    );
  }

  return (
    <div className="flex-1 space-y-4 overflow-y-auto p-4">
      {messages.map((msg, i) => (
        <MessageBubble
          key={i}
          message={msg}
          toolStatus={
            i === messages.length - 1 && msg.role === "assistant"
              ? toolStatus
              : []
          }
          orchSteps={
            i === messages.length - 1 && msg.role === "assistant"
              ? orchSteps
              : []
          }
          isStreaming={
            i === messages.length - 1 &&
            msg.role === "assistant" &&
            isStreaming
          }
        />
      ))}
      <div ref={bottomRef} />
    </div>
  );
}
