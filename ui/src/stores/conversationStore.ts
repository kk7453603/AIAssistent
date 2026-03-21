import { create } from "zustand";
import { persist } from "zustand/middleware";

export interface Conversation {
  id: string;
  title: string;
  createdAt: number;
}

function generateId(): string {
  return `conv_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
}

interface ConversationState {
  conversations: Conversation[];
  activeId: string | null;

  setActiveId: (id: string | null) => void;
  createConversation: () => string;
  deleteConversation: (id: string) => void;
  updateTitle: (id: string, title: string) => void;
}

export const useConversationStore = create<ConversationState>()(
  persist(
    (set, get) => ({
      conversations: [],
      activeId: null,

      setActiveId: (id) => set({ activeId: id }),

      createConversation: () => {
        const conv: Conversation = {
          id: generateId(),
          title: "New chat",
          createdAt: Date.now(),
        };
        set((s) => ({
          conversations: [conv, ...s.conversations],
          activeId: conv.id,
        }));
        return conv.id;
      },

      deleteConversation: (id) => {
        const { conversations, activeId } = get();
        const remaining = conversations.filter((c) => c.id !== id);
        set({
          conversations: remaining,
          activeId: activeId === id ? (remaining[0]?.id ?? null) : activeId,
        });
      },

      updateTitle: (id, title) =>
        set((s) => ({
          conversations: s.conversations.map((c) =>
            c.id === id ? { ...c, title } : c,
          ),
        })),
    }),
    { name: "paa-conversations" },
  ),
);
