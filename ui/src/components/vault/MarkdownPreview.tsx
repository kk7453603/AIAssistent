import { ExternalLink } from "lucide-react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeHighlight from "rehype-highlight";
import { useVaultStore } from "../../stores/vaultStore";
import { isTauri } from "../../utils/isTauri";

function extractFrontMatter(
  content: string,
): { meta: Record<string, string>; body: string } | null {
  const match = content.match(/^---\n([\s\S]*?)\n---\n([\s\S]*)$/);
  if (!match) return null;

  const meta: Record<string, string> = {};
  for (const line of match[1].split("\n")) {
    const colonIdx = line.indexOf(":");
    if (colonIdx > 0) {
      const key = line.slice(0, colonIdx).trim();
      const value = line.slice(colonIdx + 1).trim();
      meta[key] = value;
    }
  }
  return { meta, body: match[2] };
}

function renderWikiLinks(content: string): string {
  return content.replace(/\[\[([^\]]+)\]\]/g, "**$1**");
}

function buildObsidianUri(vaultName: string, filePath: string, vaultsPath: string): string {
  // In Tauri mode, filePath is absolute — derive relative path from vaultsPath/vaultName
  // In browser mode, filePath is already relative within the vault
  let relativePath: string;
  if (isTauri) {
    const prefix = `${vaultsPath}/${vaultName}/`;
    relativePath = filePath.startsWith(prefix) ? filePath.slice(prefix.length) : filePath;
  } else {
    relativePath = filePath;
  }
  return `obsidian://open?vault=${encodeURIComponent(vaultName)}&file=${encodeURIComponent(relativePath)}`;
}

export function MarkdownPreview() {
  const { selectedVault, selectedFilePath, fileContent, vaultsPath } = useVaultStore();

  if (!selectedFilePath || fileContent === null) {
    return (
      <div className="flex h-full items-center justify-center text-gray-400 dark:text-gray-500">
        <p>Select a file to preview</p>
      </div>
    );
  }

  const fileName = selectedFilePath.split("/").pop() ?? selectedFilePath;
  const breadcrumb = selectedFilePath
    .split("/")
    .slice(-3)
    .join(" / ");

  const parsed = extractFrontMatter(fileContent);
  const body = parsed ? parsed.body : fileContent;
  const meta = parsed?.meta;

  const obsidianUri = selectedVault
    ? buildObsidianUri(selectedVault, selectedFilePath, vaultsPath)
    : null;

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <div className="shrink-0 border-b border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-900 px-4 py-2">
        <div className="flex items-center justify-between">
          <div className="min-w-0">
            <p className="text-xs text-gray-500">{breadcrumb}</p>
            <h2 className="text-sm font-medium text-gray-800 dark:text-gray-200">{fileName}</h2>
          </div>
          {obsidianUri && (
            <button
              onClick={() => { window.location.href = obsidianUri; }}
              title="Open in Obsidian"
              className="ml-2 shrink-0 inline-flex items-center gap-1 rounded px-2 py-1 text-xs text-gray-500 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700 hover:text-gray-700 dark:hover:text-gray-200 transition-colors"
            >
              <ExternalLink className="h-3.5 w-3.5" />
              <span className="hidden sm:inline">Open in Obsidian</span>
            </button>
          )}
        </div>
        {meta && Object.keys(meta).length > 0 && (
          <div className="mt-1 flex flex-wrap gap-1">
            {Object.entries(meta).map(([k, v]) => (
              <span
                key={k}
                className="inline-flex rounded bg-gray-100 dark:bg-gray-800 px-2 py-0.5 text-xs text-gray-500 dark:text-gray-400"
              >
                {k}: {v}
              </span>
            ))}
          </div>
        )}
      </div>
      <div className="flex-1 overflow-y-auto p-4">
        <div className="prose dark:prose-invert prose-sm max-w-none">
          <Markdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeHighlight]}>
            {renderWikiLinks(body)}
          </Markdown>
        </div>
      </div>
    </div>
  );
}
