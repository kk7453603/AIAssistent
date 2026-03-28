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

// --- Orchestration Stepper ---

export interface OrchestrationStepEvent {
  type: "orchestration_step";
  orchestration_id: string;
  step_index: number;
  agent_name: string;
  task: string;
  status: "started" | "completed" | "failed";
  result: string;
  duration_ms: number;
}

// --- Scheduled Tasks ---

export interface ScheduledTask {
  id: string;
  user_id: string;
  cron_expr: string;
  prompt: string;
  condition: string;
  webhook_url: string;
  enabled: boolean;
  last_run_at: string | null;
  last_result: string;
  last_status: string;
  created_at: string;
  updated_at: string;
}

// --- Self-Improving Agent ---

export interface EventSummary {
  [eventType: string]: number;
}

export interface FeedbackSummary {
  counts: { [rating: string]: number };
  recent: AgentFeedback[];
}

export interface AgentFeedback {
  id: string;
  user_id: string;
  conversation_id: string;
  message_id: string;
  rating: string;
  comment: string;
  created_at: string;
}

export interface AgentImprovement {
  id: string;
  category: string;
  description: string;
  action: Record<string, unknown>;
  status: string;
  created_at: string;
  applied_at: string | null;
}

// --- HTTP Tools ---

export interface HTTPToolDef {
  name: string;
  description: string;
  url: string;
  method: string;
  params: Record<string, string> | null;
  body_template: Record<string, unknown> | null;
  headers: Record<string, string> | null;
  output_path: string;
  timeout_seconds: number;
}
