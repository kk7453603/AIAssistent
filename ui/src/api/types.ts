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
