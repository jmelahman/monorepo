export const queryKeys = {
  boards: ["boards"] as const,
  board: (boardId: number) => ["board", boardId] as const,
  boardEnv: (boardId: number) => ["boardEnv", boardId] as const,
  archived: (boardId: number) => ["archived", boardId] as const,
  tasks: (sessionId: number) => ["tasks", sessionId] as const,
  runs: (sessionId: number) => ["runs", sessionId] as const,
  ports: (sessionId: number) => ["ports", sessionId] as const,
  previews: (sessionId: number) => ["previews", sessionId] as const,
  allPreviews: ["previews", "all"] as const,
  previewStorage: ["previews", "storage"] as const,
  previewRetention: ["previews", "retention"] as const,
  prDetail: (sessionId: number) => ["pr-detail", sessionId] as const,
  settings: ["settings"] as const,
  config: (scope = "effective") => ["config", scope] as const,
  harnesses: ["harnesses"] as const,
  version: ["version"] as const,
  sessionPlans: (sessionId: number) => ["sessionPlans", sessionId] as const,
  sessionPlan: (sessionId: number, name: string) => ["sessionPlan", sessionId, name] as const,
  sessionDiff: (sessionId: number) => ["sessionDiff", sessionId] as const,
  sessionFile: (sessionId: number, path: string) => ["sessionFile", sessionId, path] as const,
  // Keyed by the file's diff signature so the base+head contents refetch (and
  // the diff is rebuilt) whenever the file changes, but stay cached across the
  // 4s diff poll while it doesn't.
  sessionFileDiff: (sessionId: number, path: string, signature: string) =>
    ["sessionFileDiff", sessionId, path, signature] as const,
};
