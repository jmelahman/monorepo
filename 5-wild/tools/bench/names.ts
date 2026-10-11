/**
 * A label spells a model the way its provider's API does, which is right for a
 * results directory and wrong for a title. One absent from here is titled by
 * its id, which is ugly and true.
 *
 * Its own module because two tools read it and neither can import the other:
 * `upload.ts` titles a film with it and `charts.ts` labels a bar with it, and
 * both are scripts that run on import.
 */
export const NAMES: Record<string, string> = {
  "claude-opus-5-5": "Claude Opus 5.5",
  "claude-opus-5": "Claude Opus 5",
  "claude-sonnet-5-5": "Claude Sonnet 5.5",
  "claude-haiku-5-5": "Claude Haiku 5.5",
  "gpt-6-luna": "GPT-6 Luna",
  "kimi-k3": "Kimi K3",
  "step-5-preview-free": "Step 5 Preview",
  "longcat-2.5-preview-free": "Longcat 2.5 Preview",
  "glm-5.3": "GLM 5.3",
  "deepseek-v4-pro": "DeepSeek V4 Pro",
  "minimax-m3": "MiniMax M3",
  "google/gemini-3.8-flash": "Gemini 3.8 Flash",
  "openai/gpt-6-sol": "GPT-6 Sol",
  "openai/gpt-6.1-sol": "GPT-6.1 Sol",
  "z-ai/glm-5.3-flash": "GLM 5.3 Flash",
  "xiaomi/mimo-v2.6-pro": "MiMo-V2.6-Pro",
  "deepseek/deepseek-v4.1-flash": "DeepSeek V4.1 Flash",
  "nvidia/nemotron-3-ultra-550b-a55b:free": "Nemotron 3 Ultra",
  "poolside/laguna-s-2.1:free": "Laguna S 2.1",
}

/** The model a label names: `gpt-6-luna / opencode-go` is GPT-6 Luna. */
export const modelName = (label: string): string => {
  const id = label.split(" / ")[0] ?? label
  return NAMES[id] ?? id
}
