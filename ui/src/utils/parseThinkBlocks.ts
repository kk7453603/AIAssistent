export interface ThinkBlockResult {
  thinking: string;
  answer: string;
}

export function parseThinkBlocks(content: string): ThinkBlockResult {
  // Collect all completed <think>...</think> blocks
  const thinkParts: string[] = [];
  let remaining = content;

  const regex = /<think>([\s\S]*?)<\/think>/g;
  let match;
  while ((match = regex.exec(content)) !== null) {
    const part = match[1].trim();
    if (part) thinkParts.push(part);
  }

  // Remove all think blocks from content to get the answer
  remaining = content.replace(/<think>[\s\S]*?<\/think>\s*/g, "").trim();

  // Handle streaming: <think> opened but not yet closed
  if (remaining.includes("<think>")) {
    const openMatch = remaining.match(/<think>([\s\S]*)/);
    if (openMatch) {
      const streamingThought = openMatch[1].trim();
      if (streamingThought) thinkParts.push(streamingThought);
      remaining = remaining.replace(/<think>[\s\S]*/, "").trim();
    }
  }

  return {
    thinking: thinkParts.join("\n\n"),
    answer: remaining,
  };
}
