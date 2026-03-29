import { useCallback, useState } from "react";
import {
  ChevronDown,
  ChevronRight,
  File,
  FileText,
  Folder,
  FolderOpen,
} from "lucide-react";
import { useVaultStore, type VaultEntry } from "../../stores/vaultStore";

interface FileTreeNodeProps {
  entry: VaultEntry;
  depth: number;
  onFileClick: (path: string) => void;
  onContextMenu: (e: React.MouseEvent, entry: VaultEntry) => void;
}

function FileTreeNode({
  entry,
  depth,
  onFileClick,
  onContextMenu,
}: FileTreeNodeProps) {
  const { expandedDirs, loadDir } = useVaultStore();
  const children = expandedDirs[entry.path];

  // Auto-expand if directory was pre-loaded (e.g. by showDocument navigation)
  const [expanded, setExpanded] = useState(!!children);

  const toggleExpand = useCallback(async () => {
    if (!expanded && !children) {
      await loadDir(entry.path);
    }
    setExpanded(!expanded);
  }, [expanded, children, loadDir, entry.path]);

  const handleClick = () => {
    if (entry.is_dir) {
      toggleExpand();
    } else {
      onFileClick(entry.path);
    }
  };

  const Icon = entry.is_dir
    ? expanded
      ? FolderOpen
      : Folder
    : entry.name.endsWith(".md")
      ? FileText
      : File;

  const iconColor = entry.is_dir
    ? "text-yellow-400"
    : entry.name.endsWith(".md")
      ? "text-blue-400"
      : "text-gray-400 dark:text-gray-500";

  return (
    <div>
      <div
        className="flex cursor-pointer items-center gap-1 rounded px-2 py-1 text-sm hover:bg-gray-100 dark:hover:bg-gray-800"
        style={{ paddingLeft: `${depth * 16 + 8}px` }}
        onClick={handleClick}
        onContextMenu={(e) => onContextMenu(e, entry)}
      >
        {entry.is_dir ? (
          expanded ? (
            <ChevronDown className="h-3.5 w-3.5 shrink-0 text-gray-400 dark:text-gray-500" />
          ) : (
            <ChevronRight className="h-3.5 w-3.5 shrink-0 text-gray-400 dark:text-gray-500" />
          )
        ) : (
          <span className="w-3.5 shrink-0" />
        )}
        <Icon className={`h-4 w-4 shrink-0 ${iconColor}`} />
        <span className="truncate text-gray-700 dark:text-gray-300">{entry.name}</span>
      </div>
      {expanded && children && (
        <div>
          {children.map((child) => (
            <FileTreeNode
              key={child.path}
              entry={child}
              depth={depth + 1}
              onFileClick={onFileClick}
              onContextMenu={onContextMenu}
            />
          ))}
        </div>
      )}
    </div>
  );
}

interface ContextMenuState {
  x: number;
  y: number;
  entry: VaultEntry;
}

interface Props {
  entries: VaultEntry[];
  onReferenceInChat: (path: string) => void;
}

export function FileTree({ entries, onReferenceInChat }: Props) {
  const { selectFile } = useVaultStore();
  const [contextMenu, setContextMenu] = useState<ContextMenuState | null>(
    null,
  );

  const handleContextMenu = useCallback(
    (e: React.MouseEvent, entry: VaultEntry) => {
      if (entry.is_dir) return;
      e.preventDefault();
      setContextMenu({ x: e.clientX, y: e.clientY, entry });
    },
    [],
  );

  const handleCloseMenu = useCallback(() => setContextMenu(null), []);

  return (
    <div onClick={handleCloseMenu} className="relative">
      {entries.map((entry) => (
        <FileTreeNode
          key={entry.path}
          entry={entry}
          depth={0}
          onFileClick={(path) => selectFile(path)}
          onContextMenu={handleContextMenu}
        />
      ))}

      {contextMenu && (
        <div
          className="fixed z-50 min-w-[160px] rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 py-1 shadow-xl"
          style={{ left: contextMenu.x, top: contextMenu.y }}
        >
          <button
            className="w-full px-3 py-1.5 text-left text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700"
            onClick={() => {
              onReferenceInChat(contextMenu.entry.path);
              setContextMenu(null);
            }}
          >
            Reference in chat
          </button>
          <button
            className="w-full px-3 py-1.5 text-left text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700"
            onClick={() => {
              navigator.clipboard.writeText(contextMenu.entry.path);
              setContextMenu(null);
            }}
          >
            Copy path
          </button>
        </div>
      )}
    </div>
  );
}
