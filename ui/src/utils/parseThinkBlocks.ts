export interface ThinkBlockResult {
  thinking: string;
  answer: string;
}

export function parseThinkBlocks(content: string): ThinkBlockResult {
  // Completed think block: <think>...</think>
  const closedMatch = content.match(/<think>([\s\S]*?)<\/think>\s*([\s\S]*)/);
  if (closedMatch) {
    return { thinking: closedMatch[1].trim(), answer: closedMatch[2].trim() };
  }

  // Streaming: <think> opened but not yet closed
  const openMatch = content.match(/<think>([\s\S]*)/);
  if (openMatch) {
    return { thinking: openMatch[1].trim(), answer: "" };
  }

  return { thinking: "", answer: content };
}
