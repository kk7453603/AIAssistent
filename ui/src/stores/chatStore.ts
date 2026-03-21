import { create } from "zustand";
import { persist } from "zustand/middleware";
import { getApiUrl } from "../api/client";
import type { ChatMessage, ToolStatusDelta } from "../api/types";
import { useDashboardStore } from "./dashboardStore";

interface ChatState {
  /** Messages for each conversation, keyed by conversation ID. */
  messagesByConversation: Record<string, ChatMessage[]>;
  /** Currently visible messages (derived from active conversation). */
  messages: ChatMessage[];
  activeConversationId: string | null;
  isStreaming: boolean;
  toolStatus: ToolStatusDelta[];

  /** Switch to a conversation (loads its messages). */
  loadConversation: (conversationId: string) => void;
  sendMessage: (
    content: string,
    conversationId: string,
    model: string,
  ) => Promise<void>;
  stopStreaming: () => void;
  clearMessages: () => void;
  deleteConversationMessages: (conversationId: string) => void;
}

let abortController: AbortController | null = null;

export const useChatStore = create<ChatState>()(
  persist(
    (set, get) => ({
      messagesByConversation: {},
      messages: [],
      activeConversationId: null,
      isStreaming: false,
      toolStatus: [],

      loadConversation: (conversationId) => {
        const saved = get().messagesByConversation[conversationId] ?? [];
        set({
          messages: saved,
          activeConversationId: conversationId,
          toolStatus: [],
        });
      },

      sendMessage: async (content, conversationId, model) => {
        const userMsg: ChatMessage = { role: "user", content };
        const allMessages = [...get().messages, userMsg];

        set({
          messages: allMessages,
          activeConversationId: conversationId,
          isStreaming: true,
          toolStatus: [],
        });

        // Persist user message immediately
        set((s) => ({
          messagesByConversation: {
            ...s.messagesByConversation,
            [conversationId]: allMessages,
          },
        }));

        abortController = new AbortController();

        try {
          const response = await fetch(`${getApiUrl()}/v1/chat/completions`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            signal: abortController.signal,
            body: JSON.stringify({
              model,
              messages: allMessages,
              stream: true,
              metadata: {
                user_id: "desktop",
                conversation_id: conversationId,
              },
            }),
          });

          if (!response.ok) {
            const errText = await response.text().catch(() => "");
            const errorMsg: ChatMessage = {
              role: "assistant",
              content: `Error: ${response.status} ${errText || response.statusText}`,
            };
            set((s) => {
              const updated = [...s.messages, errorMsg];
              return {
                messages: updated,
                isStreaming: false,
                messagesByConversation: {
                  ...s.messagesByConversation,
                  [conversationId]: updated,
                },
              };
            });
            return;
          }

          const reader = response.body!.getReader();
          const decoder = new TextDecoder();
          let assistantContent = "";

          // eslint-disable-next-line no-constant-condition
          while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            const chunk = decoder.decode(value, { stream: true });
            for (const line of chunk.split("\n")) {
              if (!line.startsWith("data: ")) continue;
              const data = line.slice(6);
              if (data === "[DONE]") break;

              try {
                const parsed = JSON.parse(data);
                const delta = parsed.choices?.[0]?.delta;

                if (delta?.tool_status) {
                  const ts: ToolStatusDelta = JSON.parse(delta.tool_status);
                  set((s) => ({ toolStatus: [...s.toolStatus, ts] }));
                  useDashboardStore.getState().addActivity(ts);
                  continue;
                }

                if (delta?.content) {
                  assistantContent += delta.content;
                  const snapshot = assistantContent;
                  set((s) => {
                    const msgs = [...s.messages];
                    const last = msgs[msgs.length - 1];
                    if (last?.role === "assistant") {
                      return {
                        messages: [
                          ...msgs.slice(0, -1),
                          { ...last, content: snapshot },
                        ],
                      };
                    }
                    return {
                      messages: [
                        ...msgs,
                        { role: "assistant", content: snapshot },
                      ],
                    };
                  });
                }
              } catch {
                // skip malformed SSE lines
              }
            }
          }
        } catch (err) {
          if (err instanceof DOMException && err.name === "AbortError") {
            // user cancelled
          } else {
            set((s) => ({
              messages: [
                ...s.messages,
                {
                  role: "assistant",
                  content: `Error: ${err instanceof Error ? err.message : String(err)}`,
                },
              ],
            }));
          }
        } finally {
          // Persist final messages
          set((s) => ({
            isStreaming: false,
            messagesByConversation: {
              ...s.messagesByConversation,
              [conversationId]: s.messages,
            },
          }));
          abortController = null;
        }
      },

      stopStreaming: () => {
        abortController?.abort();
      },

      clearMessages: () => set({ messages: [], toolStatus: [] }),

      deleteConversationMessages: (conversationId) => {
        set((s) => {
          const { [conversationId]: _, ...rest } = s.messagesByConversation;
          return { messagesByConversation: rest };
        });
      },
    }),
    {
      name: "paa-chat-messages",
      partialize: (s) => ({
        messagesByConversation: s.messagesByConversation,
      }),
    },
  ),
);
