// Typed client for the AgileCBT REST API. Shapes mirror the Go JSON in
// internal/db, internal/app and internal/api.

export type Lane = "someday" | "week" | "today" | "done" | "let_go";
export type GoalStatus = "active" | "resting" | "done";
export type CheckinKind = "morning" | "evening" | "adhoc";
// Topic marks a conversation that isn't a daily standup.
export type CheckinTopic = "" | "roadmap";

export interface Value {
  id: number;
  name: string;
  description: string;
  color: string;
  sort_order: number;
  created_at: string;
}

export interface Goal {
  id: number;
  value_id: number | null;
  title: string;
  why: string;
  horizon: string;
  status: GoalStatus;
  created_at: string;
  updated_at: string;
}

export interface Step {
  id: number;
  goal_id: number | null;
  title: string;
  notes: string;
  energy_cost: number;
  lane: Lane;
  sort_order: number;
  lane_changed_at: string;
  completed_at: string | null;
  predicted_pleasure: number | null;
  mastery: number | null;
  pleasure: number | null;
  created_at: string;
  updated_at: string;
}

export interface TodayStep extends Step {
  carried_over: boolean;
}

export interface Week {
  id: number;
  start_date: string;
  intention: string;
}

export interface Checkin {
  id: number;
  date: string;
  kind: CheckinKind;
  mood: number | null;
  energy: number | null;
  anxiety: number | null;
  note: string;
  summary: string;
  topic: CheckinTopic;
  created_at: string;
  updated_at: string;
}

export interface Message {
  id: number;
  checkin_id: number;
  seq: number;
  role: "user" | "assistant";
  text: string;
  created_at: string;
}

export interface Action {
  id: number;
  checkin_id: number | null;
  source: string;
  tool: string;
  summary: string;
  entity: string;
  entity_id: number;
  created_at: string;
  undone_at: string | null;
}

export interface CheckinDetail extends Checkin {
  messages: Message[];
  actions: Action[];
}

export interface Emotion {
  name: string;
  intensity: number;
}

export interface ThoughtRecord {
  id: number;
  situation: string;
  emotions: Emotion[];
  automatic_thought: string;
  distortions: string[];
  evidence_for: string;
  evidence_against: string;
  balanced_thought: string;
  rerated_emotions: Emotion[];
  created_at: string;
  updated_at: string;
}

export interface Retro {
  id: number;
  week_id: number;
  went_well: string;
  was_hard: string;
  try_next: string;
  ai_draft: string;
  created_at: string;
  updated_at: string;
}

export interface WeekReview {
  week: Week;
  end_date: string;
  checkins: Checkin[];
  completed: Step[];
  thought_records: ThoughtRecord[];
  retro: Retro | null;
}

export interface Snapshot {
  date: string;
  morning: Checkin | null;
  evening: Checkin | null;
  today: TodayStep[];
  done_today: Step[];
  week: Week;
  week_steps: Step[];
}

export interface DayMood {
  date: string;
  mood: number | null;
  energy: number | null;
  anxiety: number | null;
}

export interface Note {
  id: number;
  text: string;
  created_at: string;
}

export interface Health {
  status: string;
  version: string;
  auth_required: boolean;
  authenticated: boolean;
  llm: { backend: string; available: boolean; detail?: string };
}

export interface LLMFields {
  llm: "openai" | "none";
  base_url: string;
  model: string;
  /** Sent as reasoning_effort; "" leaves it out. */
  reasoning_effort: string;
  api_key_set: boolean;
}

/** The coach's model: config.toml/env defaults plus overrides saved here. */
export interface LLMSettings {
  effective: LLMFields;
  defaults: LLMFields;
  overrides: Partial<Record<"llm" | "base_url" | "model" | "reasoning_effort", string>>;
  api_key_saved: boolean;
}

/** A missing field is unchanged, null reverts to the default. */
export type LLMPatch = Partial<
  Record<"llm" | "base_url" | "model" | "reasoning_effort" | "api_key", string | null>
>;

export type Settings = {
  checkin_times: "morning" | "evening" | "both";
} & Record<string, string>;

export type StepPatch = Partial<
  Pick<Step, "title" | "notes" | "energy_cost" | "predicted_pleasure" | "goal_id">
> & { clear_goal?: boolean };

export type GoalPatch = Partial<Pick<Goal, "title" | "why" | "horizon" | "status" | "value_id">> & {
  clear_value?: boolean;
};

export type CheckinPatch = Partial<
  Pick<Checkin, "kind" | "mood" | "energy" | "anxiety" | "note" | "summary" | "date" | "topic">
>;

export type ThoughtPatch = Partial<Omit<ThoughtRecord, "id" | "created_at" | "updated_at">>;

export type RetroPatch = Partial<Pick<Retro, "went_well" | "was_hard" | "try_next" | "ai_draft">>;

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

// Called on any 401 so the app can show the login screen.
let onUnauthorized: () => void = () => {};
export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn;
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`/api${path}`, {
    method,
    headers: body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!res.ok) {
    let msg = res.statusText;
    try {
      const data = await res.json();
      if (data?.error) msg = data.error;
    } catch {
      // not JSON
    }
    if (res.status === 401 && path !== "/login") onUnauthorized();
    throw new ApiError(res.status, msg);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

const get = <T>(path: string) => request<T>("GET", path);
const post = <T>(path: string, body: unknown = {}) => request<T>("POST", path, body);
const patch = <T>(path: string, body: unknown) => request<T>("PATCH", path, body);
const put = <T>(path: string, body: unknown) => request<T>("PUT", path, body);
const del = (path: string) => request<void>("DELETE", path);

export const api = {
  health: () => get<Health>("/health"),
  login: (secret: string) => post<void>("/login", { secret }),
  logout: () => post<void>("/logout"),

  today: () => get<Snapshot>("/today"),
  mood: (days = 14) => get<DayMood[]>(`/mood?days=${days}`),

  values: () => get<Value[]>("/values"),
  createValue: (v: Partial<Pick<Value, "name" | "description" | "color">>) =>
    post<Value>("/values", v),
  updateValue: (id: number, v: Partial<Pick<Value, "name" | "description" | "color">>) =>
    patch<Value>(`/values/${id}`, v),
  deleteValue: (id: number) => del(`/values/${id}`),

  goals: () => get<Goal[]>("/goals"),
  createGoal: (g: GoalPatch) => post<Goal>("/goals", g),
  updateGoal: (id: number, g: GoalPatch) => patch<Goal>(`/goals/${id}`, g),

  steps: (lanes?: Lane[]) => get<Step[]>(lanes ? `/steps?lane=${lanes.join(",")}` : "/steps"),
  createStep: (lane: Lane, s: StepPatch & { title: string }) =>
    post<Step>("/steps", { ...s, lane }),
  updateStep: (id: number, s: StepPatch & { lane?: Lane; index?: number }) =>
    patch<Step>(`/steps/${id}`, s),
  moveStep: (id: number, lane: Lane, index?: number) =>
    patch<Step>(`/steps/${id}`, { lane, index }),
  completeStep: (id: number, mastery?: number, pleasure?: number) =>
    post<Step>(`/steps/${id}/complete`, { mastery, pleasure }),

  weeks: () => get<Week[]>("/weeks"),
  currentWeek: () => get<WeekReview>("/weeks/current"),
  weekReview: (id: number) => get<WeekReview>(`/weeks/${id}/review`),
  setIntention: (id: number, intention: string) => patch<Week>(`/weeks/${id}`, { intention }),
  putRetro: (weekId: number, r: RetroPatch) => put<Retro>(`/weeks/${weekId}/retro`, r),
  draftRetro: (weekId: number) => post<Retro>(`/weeks/${weekId}/retro/draft`),

  createCheckin: (c: CheckinPatch & { intro?: Pick<Message, "role" | "text">[] }) =>
    post<Checkin>("/checkins", c),
  checkin: (id: number) => get<CheckinDetail>(`/checkins/${id}`),
  updateCheckin: (id: number, c: CheckinPatch) => patch<Checkin>(`/checkins/${id}`, c),
  roadmapConversation: () => get<Checkin | null>("/conversations/roadmap"),

  thoughts: () => get<ThoughtRecord[]>("/thoughts"),
  createThought: (t: ThoughtPatch) => post<ThoughtRecord>("/thoughts", t),
  updateThought: (id: number, t: ThoughtPatch) => patch<ThoughtRecord>(`/thoughts/${id}`, t),
  deleteThought: (id: number) => del(`/thoughts/${id}`),

  notes: () => get<Note[]>("/notes"),
  createNote: (text: string) => post<Note>("/notes", { text }),
  updateNote: (id: number, text: string) => patch<Note>(`/notes/${id}`, { text }),
  deleteNote: (id: number) => del(`/notes/${id}`),

  settings: () => get<Settings>("/settings"),
  updateSettings: (s: Partial<Settings>) => patch<Settings>("/settings", s),
  llm: () => get<LLMSettings>("/llm"),
  updateLLM: (p: LLMPatch) => patch<LLMSettings>("/llm", p),
  llmModels: () => get<string[]>("/llm/models"),
  support: () => get<{ crisis_resources: string }>("/support"),

  undo: (actionId: number) => post<Action>(`/ai-actions/${actionId}/undo`),

  exportData: () => get<unknown>("/export"),
  importData: (dump: unknown) => post<{ status: string }>("/import", dump),
};

export type ChatEvent =
  | { event: "text"; data: { text: string } }
  | { event: "action"; data: Action }
  | { event: "error"; data: { error: string } }
  | { event: "done"; data: Record<string, never> };

// chat sends one message to the coach and streams the SSE reply.
export async function chat(
  checkinId: number,
  text: string,
  onEvent: (ev: ChatEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  const res = await fetch(`/api/checkins/${checkinId}/messages`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "text/event-stream" },
    body: JSON.stringify({ text }),
    signal,
  });
  if (!res.ok || !res.body) {
    let msg = res.statusText;
    try {
      msg = (await res.json()).error ?? msg;
    } catch {
      // not JSON
    }
    if (res.status === 401) onUnauthorized();
    throw new ApiError(res.status, msg);
  }
  const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
  let buf = "";
  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    buf += value;
    let sep = buf.indexOf("\n\n");
    while (sep >= 0) {
      const block = buf.slice(0, sep);
      buf = buf.slice(sep + 2);
      let event = "message";
      const data: string[] = [];
      for (const line of block.split("\n")) {
        if (line.startsWith("event:")) event = line.slice(6).trim();
        else if (line.startsWith("data:")) data.push(line.slice(5).trimStart());
      }
      if (data.length) onEvent({ event, data: JSON.parse(data.join("\n")) } as ChatEvent);
      sep = buf.indexOf("\n\n");
    }
  }
}
