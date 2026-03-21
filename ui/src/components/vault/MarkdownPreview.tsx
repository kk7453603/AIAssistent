import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeHighlight from "rehype-highlight";
import { useVaultStore } from "../../stores/vaultStore";

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

export function MarkdownPreview() {
  const { selectedFilePath, fileContent } = useVaultStore();

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

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <div className="shrink-0 border-b border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-900 px-4 py-2">
        <p className="text-xs text-gray-500">{breadcrumb}</p>
        <h2 className="text-sm font-medium text-gray-800 dark:text-gray-200">{fileName}</h2>
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
