import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type KeyboardEvent, type ReactNode, useRef, useState } from "react";
import { api, type Health, type LLMPatch, type LLMSettings, type Note } from "@/api/client";
import { Button, Dialog, ErrorText, inputClass, Markdownish, SectionTitle } from "@/components/ui";
import {
  loadTheme,
  saveTheme,
  THEME_ACCENTS,
  THEME_MODES,
  THEME_STYLES,
  type Theme,
} from "@/theme";

const tabs = [
  { id: "coach", label: "Coach" },
  { id: "model", label: "AI model" },
  { id: "appearance", label: "Appearance" },
  { id: "data", label: "Data" },
  { id: "support", label: "Support" },
] as const;
export type SettingsTab = (typeof tabs)[number]["id"];

export function isSettingsTab(v: string | null): v is SettingsTab {
  return tabs.some((t) => t.id === v);
}

// Settings is a dialog over whatever page you're on, split into a few tabs
// so there's only one short list to look at at a time. The open tab lives in
// the URL (?settings=coach), owned by Layout.
export default function Settings({
  health,
  tab,
  onTab,
  onClose,
}: {
  health: Health;
  tab: SettingsTab | null;
  onTab: (t: SettingsTab) => void;
  onClose: () => void;
}) {
  const refs = useRef<Record<string, HTMLButtonElement | null>>({});
  // Arrow keys move between tabs, as in any tab list.
  function onKeyDown(e: KeyboardEvent) {
    const step = { ArrowRight: 1, ArrowLeft: -1 }[e.key];
    if (!step || !tab) return;
    e.preventDefault();
    const at = tabs.findIndex((t) => t.id === tab);
    const next = tabs[(at + step + tabs.length) % tabs.length].id;
    onTab(next);
    refs.current[next]?.focus();
  }
  return (
    <Dialog open={tab !== null} onClose={onClose} title="Settings" wide>
      <div
        role="tablist"
        aria-label="Settings"
        className="-mx-5 mb-4 flex overflow-x-auto border-b border-border px-5"
        onKeyDown={onKeyDown}
      >
        {tabs.map((t) => (
          <button
            key={t.id}
            ref={(el) => {
              refs.current[t.id] = el;
            }}
            type="button"
            role="tab"
            id={`settings-tab-${t.id}`}
            aria-selected={tab === t.id}
            aria-controls="settings-panel"
            tabIndex={tab === t.id ? 0 : -1}
            onClick={() => onTab(t.id)}
            className={`ui-caps -mb-px border-b-2 px-3 py-2 text-sm ${tab === t.id ? "border-accent-500 text-fg" : "border-transparent text-fg-muted hover:text-fg"}`}
          >
            {t.label}
          </button>
        ))}
      </div>
      <div
        id="settings-panel"
        role="tabpanel"
        aria-labelledby={`settings-tab-${tab}`}
        // The padding, cancelled by the margin, keeps focus rings (up to 4px
        // out, with the swatches' offset) inside the scroll area's clip.
        className="-m-1 h-[min(30rem,65dvh)] space-y-6 overflow-y-auto p-1"
      >
        {tab === "coach" && (
          <>
            <CheckinTimes />
            <NotesEditor />
          </>
        )}
        {tab === "model" && <ModelSettings health={health} />}
        {tab === "appearance" && <Appearance />}
        {tab === "data" && (
          <>
            <DataCard />
            {health.auth_required && <Logout />}
            <p className="text-xs text-fg-muted">AgileCBT {health.version}</p>
          </>
        )}
        {tab === "support" && <Support />}
      </div>
    </Dialog>
  );
}

// ModelSettings is where the coach's LLM is chosen.
function ModelSettings({ health }: { health: Health }) {
  const { llm } = health;
  const settings = useQuery({ queryKey: ["llm"], queryFn: api.llm });
  return (
    <section>
      <p className="text-sm">
        <span aria-hidden className={llm.available ? "text-calm" : "text-fg-muted"}>
          {llm.available ? "●" : "○"}
        </span>{" "}
        <span>
          {llm.backend === "none" ? "Turned off" : llm.available ? "Ready" : "Unavailable"}
        </span>
      </p>
      {llm.detail && llm.backend !== "none" && (
        <p className="mt-1 text-xs text-fg-muted">{llm.detail}</p>
      )}
      {settings.data ? (
        // Remount on save so the form restarts from what's stored.
        <ModelForm key={JSON.stringify(settings.data)} settings={settings.data} />
      ) : (
        <ErrorText error={settings.error} />
      )}
      <p className="mt-4 text-xs text-fg-muted">
        Connect Claude Code or Claude Desktop to <code>{window.location.origin}/mcp</code> to plan
        with the same tools. See the AI guide in the docs.
      </p>
    </section>
  );
}

const efforts = [
  { value: "none", label: "Off" },
  { value: "low", label: "Low" },
  { value: "medium", label: "Medium" },
  { value: "high", label: "High" },
  { value: "", label: "Don't send" },
];

const effortLabel = (v: string) => efforts.find((e) => e.value === v)?.label ?? v;

// ModelForm edits the settings saved in the app. Blank fields fall back to
// config.toml / the environment, shown as placeholders.
function ModelForm({ settings: s }: { settings: LLMSettings }) {
  const qc = useQueryClient();
  const d = s.defaults;
  const [on, setOn] = useState((s.overrides.llm ?? d.llm) !== "none");
  const [baseURL, setBaseURL] = useState(s.overrides.base_url ?? "");
  const [model, setModel] = useState(s.overrides.model ?? "");
  // null follows the default.
  const [effort, setEffort] = useState<string | null>(s.overrides.reasoning_effort ?? null);
  const [key, setKey] = useState("");
  const models = useQuery({
    queryKey: ["llm-models", s.effective.base_url, s.effective.api_key_set],
    queryFn: api.llmModels,
    enabled: s.effective.llm !== "none",
    staleTime: 60_000,
  });
  const save = useMutation({
    mutationFn: api.updateLLM,
    onSuccess: (next) => {
      qc.setQueryData(["llm"], next);
      qc.invalidateQueries({ queryKey: ["health"] });
    },
  });

  const llm = on ? "openai" : "none";
  const patch: LLMPatch = {
    llm: llm === d.llm ? null : llm,
    base_url: baseURL.trim() || null,
    model: model.trim() || null,
    reasoning_effort: effort,
  };
  const dirty =
    key.trim() !== "" ||
    Object.entries(patch).some(
      ([k, v]) => v !== (s.overrides[k as keyof LLMSettings["overrides"]] ?? null),
    );
  const customized = Object.keys(s.overrides).length > 0 || s.api_key_saved;
  // A key only goes to the URL it was saved for.
  const urlChanged = (baseURL.trim().replace(/\/+$/, "") || d.base_url) !== s.effective.base_url;
  const keyHint = urlChanged
    ? "Enter the key for the new URL"
    : s.api_key_saved
      ? "Saved (leave blank to keep it)"
      : s.effective.api_key_set
        ? "Set in config"
        : "None (Ollama doesn't need one)";

  return (
    <form
      className="mt-4 space-y-3 text-sm"
      onSubmit={(e) => {
        e.preventDefault();
        save.mutate(key.trim() ? { ...patch, api_key: key.trim() } : patch);
      }}
    >
      <label className="flex items-center gap-2">
        <input type="checkbox" checked={on} onChange={(e) => setOn(e.target.checked)} />
        <span className="font-medium">Use an AI coach</span>
      </label>
      {on && (
        <>
          <Field
            id="llm-base-url"
            label="API URL"
            hint="Any OpenAI-compatible API, such as Ollama or OpenRouter."
          >
            <input
              id="llm-base-url"
              aria-describedby="llm-base-url-hint"
              value={baseURL}
              onChange={(e) => setBaseURL(e.target.value)}
              placeholder={d.base_url}
              list="llm-base-urls"
              inputMode="url"
              spellCheck={false}
              className={inputClass}
            />
            <datalist id="llm-base-urls">
              <option value="http://localhost:11434/v1">Ollama on this machine</option>
              <option value="https://openrouter.ai/api/v1">OpenRouter</option>
            </datalist>
          </Field>
          <Field id="llm-model" label="Model">
            <input
              id="llm-model"
              value={model}
              onChange={(e) => setModel(e.target.value)}
              placeholder={d.model}
              list="llm-models"
              spellCheck={false}
              className={inputClass}
            />
            <datalist id="llm-models">
              {models.data?.map((m) => (
                <option key={m} value={m} />
              ))}
            </datalist>
          </Field>
          <Field id="llm-api-key" label="API key">
            <div className="flex gap-2">
              <input
                id="llm-api-key"
                type="password"
                value={key}
                onChange={(e) => setKey(e.target.value)}
                placeholder={keyHint}
                autoComplete="off"
                className={inputClass}
              />
              {s.api_key_saved && (
                <Button
                  variant="ghost"
                  onClick={() => save.mutate({ api_key: null })}
                  disabled={save.isPending}
                >
                  Remove
                </Button>
              )}
            </div>
          </Field>
          <Field
            id="llm-effort"
            label="Thinking"
            hint="Off keeps replies quick on models like Qwen3."
          >
            <select
              id="llm-effort"
              aria-describedby="llm-effort-hint"
              value={effort ?? "default"}
              onChange={(e) => setEffort(e.target.value === "default" ? null : e.target.value)}
              className={`${selectClass} w-full py-2`}
            >
              <option value="default">Default ({effortLabel(d.reasoning_effort)})</option>
              {efforts.map((e) => (
                <option key={e.value} value={e.value}>
                  {e.label}
                </option>
              ))}
            </select>
          </Field>
        </>
      )}
      <div className="flex flex-wrap items-center gap-2 pt-1">
        <Button type="submit" variant="soft" disabled={!dirty || save.isPending}>
          {save.isPending ? "Saving…" : "Save"}
        </Button>
        {customized && (
          <Button
            variant="ghost"
            disabled={save.isPending}
            onClick={() =>
              save.mutate({
                llm: null,
                base_url: null,
                model: null,
                reasoning_effort: null,
                api_key: null,
              })
            }
          >
            Reset to config
          </Button>
        )}
      </div>
      <ErrorText error={save.error} />
    </form>
  );
}

function Field({
  id,
  label,
  hint,
  children,
}: {
  id: string;
  label: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <div className="space-y-1">
      <label htmlFor={id} className="block font-medium">
        {label}
      </label>
      {children}
      {hint && (
        <p id={`${id}-hint`} className="text-xs text-fg-muted">
          {hint}
        </p>
      )}
    </div>
  );
}

function NotesEditor() {
  const qc = useQueryClient();
  const notes = useQuery({ queryKey: ["notes"], queryFn: api.notes });
  const [text, setText] = useState("");
  const add = useMutation({
    mutationFn: () => api.createNote(text.trim()),
    onSuccess: () => {
      setText("");
      qc.invalidateQueries({ queryKey: ["notes"] });
    },
  });
  return (
    <section>
      <SectionTitle>What your coach remembers</SectionTitle>
      <p className="mb-3 text-xs text-fg-muted">
        These notes are your coach's only memory between conversations. Edit or remove anything.
      </p>
      <ul className="space-y-2">
        {notes.data?.length === 0 && <li className="text-sm text-fg-muted">Nothing yet.</li>}
        {notes.data?.map((n) => (
          <NoteRow key={n.id} note={n} />
        ))}
      </ul>
      <form
        className="mt-3 flex gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          if (text.trim()) add.mutate();
        }}
      >
        <input
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder="e.g. Mornings are hardest; start small."
          aria-label="New note"
          className={inputClass}
        />
        <Button type="submit" variant="soft" disabled={!text.trim() || add.isPending}>
          Add
        </Button>
      </form>
      <ErrorText error={notes.error ?? add.error} />
    </section>
  );
}

function NoteRow({ note }: { note: Note }) {
  const qc = useQueryClient();
  const [text, setText] = useState(note.text);
  const refresh = () => qc.invalidateQueries({ queryKey: ["notes"] });
  const save = useMutation({
    mutationFn: () => api.updateNote(note.id, text.trim()),
    onSuccess: refresh,
  });
  const remove = useMutation({ mutationFn: () => api.deleteNote(note.id), onSuccess: refresh });
  return (
    <li className="flex gap-2">
      <input
        value={text}
        onChange={(e) => setText(e.target.value)}
        onBlur={() => text.trim() && text.trim() !== note.text && save.mutate()}
        aria-label="Note"
        className={`${inputClass} py-1.5`}
      />
      <Button variant="ghost" onClick={() => remove.mutate()} aria-label={`Forget: ${note.text}`}>
        Forget
      </Button>
    </li>
  );
}

function CheckinTimes() {
  const qc = useQueryClient();
  const settings = useQuery({ queryKey: ["settings"], queryFn: api.settings });
  const update = useMutation({
    mutationFn: api.updateSettings,
    onSuccess: (s) => qc.setQueryData(["settings"], s),
  });
  if (!settings.data) return <ErrorText error={settings.error} />;
  return (
    <section className="text-sm">
      <label className="flex items-center justify-between gap-2">
        <span className="font-medium">Check-ins</span>
        <select
          value={settings.data.checkin_times}
          onChange={(e) => update.mutate({ checkin_times: e.target.value as "both" })}
          className={selectClass}
        >
          <option value="both">Morning and evening</option>
          <option value="morning">Morning only</option>
          <option value="evening">Evening only</option>
        </select>
      </label>
      <ErrorText error={update.error} />
    </section>
  );
}

// Support shows the crisis lines the coach shares. They're set with
// crisis_resources in config.toml, so they can't be changed here by accident.
function Support() {
  const support = useQuery({ queryKey: ["support"], queryFn: api.support });
  return (
    <section>
      <SectionTitle>You don't have to do this alone</SectionTitle>
      <Markdownish
        text={
          support.data?.crisis_resources ??
          "If you might act on thoughts of harming yourself, call your local emergency number. In the US, call or text 988."
        }
        className="text-sm leading-relaxed"
        links
      />
    </section>
  );
}

const selectClass = "rounded-lg border border-border bg-bg px-2 py-1";

function Appearance() {
  const [theme, setTheme] = useState<Theme>(loadTheme);
  const set = (patch: Partial<Theme>) => {
    const next = { ...theme, ...patch };
    setTheme(next);
    saveTheme(next);
  };
  return (
    <section>
      <p className="mb-4 text-xs text-fg-muted">Saved in this browser only.</p>
      <div className="space-y-4 text-sm">
        <label className="flex items-center justify-between gap-2">
          <span className="font-medium">Mode</span>
          <select
            value={theme.mode}
            onChange={(e) => set({ mode: e.target.value as Theme["mode"] })}
            className={selectClass}
          >
            {THEME_MODES.map((m) => (
              <option key={m.value} value={m.value}>
                {m.label}
              </option>
            ))}
          </select>
        </label>
        <label className="flex items-center justify-between gap-2">
          <span className="font-medium">Style</span>
          <select
            value={theme.style}
            onChange={(e) => set({ style: e.target.value as Theme["style"] })}
            className={selectClass}
          >
            {THEME_STYLES.map((s) => (
              <option key={s.value} value={s.value}>
                {s.label}
              </option>
            ))}
          </select>
        </label>
        <fieldset className="flex items-center justify-between gap-2">
          <legend className="float-left font-medium">Accent</legend>
          <div className="flex gap-2">
            {THEME_ACCENTS.map((a) => (
              <label key={a.value} title={a.label} className="relative">
                <input
                  type="radio"
                  name="accent"
                  value={a.value}
                  checked={theme.accent === a.value}
                  onChange={() => set({ accent: a.value })}
                  aria-label={a.label}
                  className="peer sr-only"
                />
                {/* Carries its own data-accent so it shows that accent, not the active one. */}
                <span
                  data-accent={a.value}
                  className="block h-7 w-7 cursor-pointer rounded-full bg-accent-600 ring-offset-2 ring-offset-surface peer-checked:ring-2 peer-checked:ring-fg peer-focus-visible:ring-2 peer-focus-visible:ring-accent-500"
                />
              </label>
            ))}
          </div>
        </fieldset>
      </div>
    </section>
  );
}

function DataCard() {
  const qc = useQueryClient();
  const [msg, setMsg] = useState<string | null>(null);
  const exp = useMutation({
    mutationFn: api.exportData,
    onSuccess: (dump) => {
      const blob = new Blob([JSON.stringify(dump, null, 2)], { type: "application/json" });
      const a = document.createElement("a");
      a.href = URL.createObjectURL(blob);
      a.download = `agilecbt-${new Date().toISOString().slice(0, 10)}.json`;
      a.click();
      URL.revokeObjectURL(a.href);
    },
  });
  const imp = useMutation({
    mutationFn: async (file: File) => api.importData(JSON.parse(await file.text())),
    onSuccess: () => {
      setMsg("Imported.");
      qc.invalidateQueries();
    },
  });
  return (
    <section>
      <p className="mb-3 text-xs text-fg-muted">
        Everything lives on your server. Export a JSON backup any time; import only works into an
        empty instance.
      </p>
      <div className="flex flex-wrap gap-2">
        <Button variant="soft" onClick={() => exp.mutate()} disabled={exp.isPending}>
          Export JSON
        </Button>
        <label className="cursor-pointer rounded-xl bg-surface-2 px-4 py-2 text-sm font-medium hover:bg-border">
          Import JSON
          <input
            type="file"
            accept="application/json,.json"
            className="sr-only"
            onChange={(e) => {
              const f = e.target.files?.[0];
              if (f) imp.mutate(f);
              e.target.value = "";
            }}
          />
        </label>
      </div>
      {msg && <p className="mt-2 text-sm text-calm">{msg}</p>}
      <ErrorText error={exp.error ?? imp.error} />
    </section>
  );
}

function Logout() {
  const qc = useQueryClient();
  const out = useMutation({ mutationFn: api.logout, onSuccess: () => qc.invalidateQueries() });
  return (
    <Button variant="ghost" className="w-full" onClick={() => out.mutate()}>
      Log out
    </Button>
  );
}
