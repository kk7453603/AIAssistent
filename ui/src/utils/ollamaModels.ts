export interface OllamaModel {
  name: string;
  size: number;
  details?: {
    family?: string;
    families?: string[];
    parameter_size?: string;
    quantization_level?: string;
  };
}

const EMBEDDING_MARKERS = [
  "embed",
  "embedding",
  "nomic-bert",
  "nomic-embed",
  "bge",
  "gte",
  "e5",
];

export function isLikelyEmbeddingModel(model: OllamaModel | string): boolean {
  const name = typeof model === "string" ? model : model.name;
  const details = typeof model === "string" ? undefined : model.details;
  const haystacks = [
    name,
    details?.family ?? "",
    ...(details?.families ?? []),
  ]
    .join(" ")
    .toLowerCase();

  return EMBEDDING_MARKERS.some((marker) => haystacks.includes(marker));
}

export function sortModelsBySize(models: OllamaModel[]): OllamaModel[] {
  return [...models].sort((a, b) => {
    if (b.size !== a.size) {
      return b.size - a.size;
    }
    return a.name.localeCompare(b.name);
  });
}

export function ensureSelectedModel(
  models: OllamaModel[],
  selected: string,
): OllamaModel[] {
  const trimmed = selected.trim();
  if (!trimmed || models.some((model) => model.name === trimmed)) {
    return models;
  }
  return [{ name: trimmed, size: 0 }, ...models];
}

export function modelKindLabel(model: OllamaModel): "Embedding" | "Chat" {
  return isLikelyEmbeddingModel(model) ? "Embedding" : "Chat";
}
