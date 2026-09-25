import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { api, type Health, type Note } from "@/api/client";
import { Button, Card, ErrorText, inputClass, SectionTitle } from "@/components/ui";
import {
  loadTheme,
  saveTheme,
  THEME_ACCENTS,
  THEME_MODES,
  THEME_STYLES,
  type Theme,
} from "@/theme";

export default function Settings({ health }: { health: Health }) {
  return (
    <div className="space-y-4">
      <h1 className="text-xl font-semibold">Settings</h1>
      <CuratorStatus health={health} />
      <NotesEditor />
      <Preferences />
      <Appearance />
      <DataCard />
      {health.auth_required && <Logout />}
      <p className="text-center text-xs text-fg-muted">AgileCBT {health.version}</p>
    </div>
  );
}

function CuratorStatus({ health }: { health: Health }) {
  const { llm } = health;
  return (
    <Card>
      <SectionTitle>AI curator</SectionTitle>
      <p className="text-sm">
        <span className={llm.available ? "text-calm" : "text-fg-muted"}>
          {llm.available ? "●" : "○"}
        </span>{" "}
        {llm.backend === "none"
          ? "Turned off"
          : `${llm.backend} — ${llm.available ? "ready" : "unavailable"}`}
      </p>
      {llm.detail && <p className="mt-1 text-xs text-fg-muted">{llm.detail}</p>}
      <p className="mt-3 text-xs text-fg-muted">
        Connect Claude Code or Claude Desktop to <code>{window.location.origin}/mcp</code> to plan
        with the same tools. See the AI guide in the docs.
      </p>
    </Card>
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
    <Card>
      <SectionTitle>What the curator remembers</SectionTitle>
      <p className="mb-3 text-xs text-fg-muted">
        These notes are the curator's only memory between check-ins. Edit or remove anything.
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
    </Card>
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

function Preferences() {
  const qc = useQueryClient();
  const settings = useQuery({ queryKey: ["settings"], queryFn: api.settings });
  const [crisis, setCrisis] = useState("");
  useEffect(() => {
    if (settings.data) setCrisis(settings.data.crisis_resources);
  }, [settings.data]);
  const update = useMutation({
    mutationFn: api.updateSettings,
    onSuccess: (s) => qc.setQueryData(["settings"], s),
  });
  if (!settings.data) return <ErrorText error={settings.error} />;
  return (
    <Card>
      <SectionTitle>Preferences</SectionTitle>
      <div className="space-y-4 text-sm">
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
        <label className="block">
          <span className="font-medium">Crisis resources</span>
          <span className="block text-xs text-fg-muted">
            Shown by “Need help now?” and given to the curator. Put your own people and local
            numbers here.
          </span>
          <textarea
            value={crisis}
            onChange={(e) => setCrisis(e.target.value)}
            rows={6}
            className={`${inputClass} mt-1`}
          />
        </label>
        <div className="flex gap-2">
          <Button
            variant="soft"
            onClick={() => update.mutate({ crisis_resources: crisis })}
            disabled={update.isPending || crisis === settings.data.crisis_resources}
          >
            Save resources
          </Button>
          <Button variant="ghost" onClick={() => update.mutate({ crisis_resources: "" })}>
            Restore default
          </Button>
        </div>
        <ErrorText error={update.error} />
      </div>
    </Card>
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
    <Card>
      <SectionTitle aside={<span className="text-xs text-fg-muted">This browser only</span>}>
        Appearance
      </SectionTitle>
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
    </Card>
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
    <Card>
      <SectionTitle>Your data</SectionTitle>
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
    </Card>
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
