import { MessageSquarePlus, Trash2 } from "lucide-react";
import { useConversationStore } from "../../stores/conversationStore";
import { useChatStore } from "../../stores/chatStore";

interface Props {
  onSelect: (id: string) => void;
  onCreate: () => void;
}

export function ConversationSidebar({ onSelect, onCreate }: Props) {
  const { conversations, activeId, deleteConversation } =
    useConversationStore();

  return (
    <div className="flex h-full flex-col">
      <div className="border-b border-gray-200 dark:border-gray-800 p-3">
        <button
          onClick={onCreate}
          className="flex w-full items-center gap-2 rounded-lg border border-gray-300 dark:border-gray-700 px-3 py-2 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800"
        >
          <MessageSquarePlus className="h-4 w-4" />
          New chat
        </button>
      </div>
      <div className="flex-1 overflow-y-auto p-2">
        {conversations.map((conv) => (
          <div
            key={conv.id}
            className={`group flex items-center justify-between rounded-lg px-3 py-2 text-sm cursor-pointer ${
              conv.id === activeId
                ? "bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-gray-100"
                : "text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800/50 hover:text-gray-700 dark:hover:text-gray-200"
            }`}
            onClick={() => onSelect(conv.id)}
          >
            <span className="truncate">{conv.title}</span>
            <button
              onClick={(e) => {
                e.stopPropagation();
                useChatStore.getState().deleteConversationMessages(conv.id);
                deleteConversation(conv.id);
              }}
              className="shrink-0 text-gray-400 dark:text-gray-600 opacity-0 transition-opacity hover:text-red-400 group-hover:opacity-100"
            >
              <Trash2 className="h-3.5 w-3.5" />
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}
