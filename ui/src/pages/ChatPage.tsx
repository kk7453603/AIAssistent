import { useCallback, useEffect, useState } from "react";
import { Menu, X } from "lucide-react";
import { ConversationSidebar } from "../components/chat/ConversationSidebar";
import { InputBar } from "../components/chat/InputBar";
import { MessageList } from "../components/chat/MessageList";
import { useChatStore } from "../stores/chatStore";
import { useConversationStore } from "../stores/conversationStore";

const DEFAULT_MODEL = "paa-agent";

interface Props {
  pendingReference?: string | null;
  onReferenceClear?: () => void;
}

export function ChatPage({ pendingReference, onReferenceClear }: Props) {
  const { conversations, activeId, setActiveId, createConversation, updateTitle } =
    useConversationStore();

  const {
    messages,
    isStreaming,
    toolStatus,
    sendMessage,
    stopStreaming,
    loadConversation,
  } = useChatStore();

  // Load messages when component mounts with an active conversation
  useEffect(() => {
    if (activeId) {
      loadConversation(activeId);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (pendingReference) {
      let convId = activeId;
      if (!convId) {
        convId = createConversation();
      }
      sendMessage(pendingReference, convId, DEFAULT_MODEL);
      onReferenceClear?.();
    }
  }, [pendingReference, activeId, createConversation, sendMessage, onReferenceClear]);

  const handleSend = useCallback(
    (content: string, model?: string) => {
      let convId = activeId;
      if (!convId) {
        convId = createConversation();
      }
      const conv = conversations.find((c) => c.id === convId);
      if (conv?.title === "New chat") {
        updateTitle(convId, content.slice(0, 50));
      }
      sendMessage(content, convId, model ?? DEFAULT_MODEL);
    },
    [activeId, conversations, createConversation, sendMessage, updateTitle],
  );

  const handleNewChat = useCallback(() => {
    const newId = createConversation();
    loadConversation(newId);
  }, [createConversation, loadConversation]);

  const handleSelectConversation = useCallback(
    (id: string) => {
      setActiveId(id);
      loadConversation(id);
    },
    [setActiveId, loadConversation],
  );

  const [sidebarOpen, setSidebarOpen] = useState(false);

  const handleSelectMobile = useCallback(
    (id: string) => {
      handleSelectConversation(id);
      setSidebarOpen(false);
    },
    [handleSelectConversation],
  );

  return (
    <div className="relative flex h-full">
      {/* Mobile sidebar overlay */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 z-20 bg-black/50 md:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      {/* Sidebar: hidden on mobile unless toggled, always visible on md+ */}
      <div
        className={`fixed inset-y-0 left-0 z-30 w-56 border-r border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-900 transition-transform md:static md:translate-x-0 md:shrink-0 ${
          sidebarOpen ? "translate-x-0" : "-translate-x-full"
        }`}
      >
        {/* Mobile close button */}
        <div className="flex items-center justify-end p-2 md:hidden">
          <button
            onClick={() => setSidebarOpen(false)}
            className="rounded-md p-1 text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-gray-700 dark:hover:text-gray-200"
          >
            <X className="h-5 w-5" />
          </button>
        </div>
        <ConversationSidebar
          onSelect={handleSelectMobile}
          onCreate={handleNewChat}
        />
      </div>

      <div className="flex min-w-0 flex-1 flex-col">
        {/* Mobile menu button */}
        <div className="flex items-center border-b border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 px-3 py-2 md:hidden">
          <button
            onClick={() => setSidebarOpen(true)}
            className="rounded-md p-1 text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-gray-700 dark:hover:text-gray-200"
          >
            <Menu className="h-5 w-5" />
          </button>
          <span className="ml-2 text-sm text-gray-500 dark:text-gray-400">Conversations</span>
        </div>
        <MessageList
          messages={messages}
          toolStatus={toolStatus}
          isStreaming={isStreaming}
        />
        <InputBar
          onSend={handleSend}
          onStop={stopStreaming}
          isStreaming={isStreaming}
        />
      </div>
    </div>
  );
}
