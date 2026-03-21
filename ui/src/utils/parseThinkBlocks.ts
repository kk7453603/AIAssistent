export interface ThinkBlockResult {
  thinking: string;
  answer: string;
}

export function parseThinkBlocks(content: string): ThinkBlockResult {
  const match = content.match(/<think>([\s\S]*?)<\/think>\s*([\s\S]*)/);
  if (!match) return { thinking: "", answer: content };
  return { thinking: match[1].trim(), answer: match[2].trim() };
}
