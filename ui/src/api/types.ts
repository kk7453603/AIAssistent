export interface ChatMessage {
  role: "system" | "user" | "assistant" | "tool";
  content: string;
}

export interface ChatCompletionRequest {
  model: string;
  messages: ChatMessage[];
  stream?: boolean;
  metadata?: {
    user_id?: string;
    conversation_id?: string;
  };
}

export interface ChatCompletionChoice {
  index: number;
  message: ChatMessage;
  finish_reason: string;
}

export interface ChatCompletionResponse {
  id: string;
  choices: ChatCompletionChoice[];
}

export interface HealthResponse {
  status: string;
}

export interface ToolStatusDelta {
  tool: string;
  status: "running" | "ok" | "error";
}

// --- Knowledge Graph ---

export interface GraphNode {
  id: string;
  filename: string;
  source_type: string;
  category: string;
  title: string;
  path: string;
}

export interface GraphRelation {
  source_id: string;
  target_id: string;
  type: string;
  weight: number;
}

export interface Graph {
  nodes: GraphNode[];
  edges: GraphRelation[];
}

export interface GraphFilter {
  source_types?: string[];
  categories?: string[];
  min_score?: number;
  max_depth?: number;
}
