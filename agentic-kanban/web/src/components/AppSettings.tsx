import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { api } from "@/api/client";
import { queryKeys } from "@/api/keys";
import { setDevToolbarPrefs, useDevToolbarPrefs } from "@/components/devToolbar/preferences";
import { ACCENTS, type Accent, setAccent, useAccent } from "@/hooks/useAccent";
import { type Contrast, setContrast, useContrast } from "@/hooks/useContrast";
import { useDevToolbarEnabled } from "@/hooks/useDevToolbarEnabled";
import { setThemeMode, type ThemeMode, useThemeMode } from "@/hooks/useThemeMode";
import {
  setTerminalOrientation,
  type TerminalOrientation,
  useTerminalOrientation,
} from "@/hooks/useTerminalOrientation";
import { useToast } from "@/toast";
import { Button } from "./Button";
import { FormField, FormInput } from "./FormField";
import { KeybindingsSettings } from "./KeybindingsSettings";
import { Modal } from "./Modal";
import { Tab } from "./Tab";

const ACCENT_LABELS: Record<Accent, string> = {
  red: "Red",
  orange: "Orange",
  amber: "Amber",
  green: "Green",
  teal: "Teal",
  blue: "Blue",
  indigo: "Indigo",
  purple: "Purple",
  pink: "Pink",
};

type SettingsTab = "general" | "appearance" | "shortcuts" | "developer";

export function AppSettings({ open, onClose }: { open: boolean; onClose: () => void }) {
  const qc = useQueryClient();
  const { push } = useToast();
  const settingsQ = useQuery({
    queryKey: queryKeys.settings,
    queryFn: api.getSettings,
    enabled: open,
  });
  const harnessesQ = useQuery({
    queryKey: queryKeys.harnesses,
    queryFn: api.listHarnesses,
    enabled: open,
  });
  const versionQ = useQuery({
    queryKey: queryKeys.version,
    queryFn: api.getVersion,
    staleTime: Infinity,
    enabled: open,
  });

  const [tab, setTab] = useState<SettingsTab>("general");
  const [harness, setHarness] = useState<string>("");
  const [worktreesRoot, setWorktreesRoot] = useState<string>("");
  const [signCommits, setSignCommits] = useState<boolean>(false);
  const savedOrientation = useTerminalOrientation();
  const [orientation, setOrientation] = useState<TerminalOrientation>(savedOrientation);
  const { mode } = useThemeMode();
  const contrast = useContrast();
  const accent = useAccent();
  const devToolbarEnabled = useDevToolbarEnabled();
  const devToolbarPrefs = useDevToolbarPrefs();

  useEffect(() => {
    if (settingsQ.data) {
      setHarness(settingsQ.data.harness);
      setWorktreesRoot(settingsQ.data.worktrees_root);
      setSignCommits(settingsQ.data.sign_commits);
    }
  }, [settingsQ.data]);

  // Re-sync the form to the persisted value each time the modal opens.
  // biome-ignore lint/correctness/useExhaustiveDependencies: only `open` toggles re-sync; the saved value is read at that moment.
  useEffect(() => {
    if (open) {
      setOrientation(savedOrientation);
      setTab("general");
    }
  }, [open]);

  const updateMut = useMutation({
    mutationFn: async () => {
      if (orientation !== savedOrientation) setTerminalOrientation(orientation);
      const payload: { harness?: string; worktrees_root?: string; sign_commits?: boolean } = {};
      if (settingsQ.data && harness !== settingsQ.data.harness) payload.harness = harness;
      if (
        settingsQ.data &&
        !settingsQ.data.worktrees_root_locked &&
        worktreesRoot.trim() !== settingsQ.data.worktrees_root
      ) {
        payload.worktrees_root = worktreesRoot.trim();
      }
      if (settingsQ.data && signCommits !== settingsQ.data.sign_commits) {
        payload.sign_commits = signCommits;
      }
      await api.updateSettings(payload);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.settings });
      push("success", "Settings saved.");
      onClose();
    },
  });

  const harnessDirty = settingsQ.data ? harness !== settingsQ.data.harness : false;
  const worktreesRootDirty = settingsQ.data
    ? !settingsQ.data.worktrees_root_locked &&
      worktreesRoot.trim() !== settingsQ.data.worktrees_root
    : false;
  const orientationDirty = orientation !== savedOrientation;
  const signCommitsDirty = settingsQ.data ? signCommits !== settingsQ.data.sign_commits : false;
  const dirty = harnessDirty || orientationDirty || worktreesRootDirty || signCommitsDirty;
  const busy = updateMut.isPending;
  const harnesses = harnessesQ.data ?? [];
  const worktreesLocked = settingsQ.data?.worktrees_root_locked ?? false;
  const worktreesResolved = settingsQ.data?.worktrees_root_resolved ?? "";

  return (
    <Modal open={open} onClose={onClose} title="Settings" busy={busy}>
      <div className="flex border-b border-border text-sm">
        <Tab active={tab === "general"} onClick={() => setTab("general")} label="general" />
        <Tab
          active={tab === "appearance"}
          onClick={() => setTab("appearance")}
          label="appearance"
        />
        <Tab active={tab === "shortcuts"} onClick={() => setTab("shortcuts")} label="shortcuts" />
        {devToolbarEnabled && (
          <Tab active={tab === "developer"} onClick={() => setTab("developer")} label="developer" />
        )}
      </div>
      <form
        className="flex min-h-[420px] flex-col gap-3 p-4 text-sm"
        onSubmit={(e) => {
          e.preventDefault();
          if (!dirty) return;
          updateMut.mutate();
        }}
      >
        {tab === "general" && (
          <>
            <FormField label="Agent harness">
              <select
                className="rounded bg-surface px-2 py-1"
                value={harness}
                onChange={(e) => setHarness(e.target.value)}
                disabled={settingsQ.isLoading || harnessesQ.isLoading}
              >
                <option value="">— use project / default —</option>
                {harnesses.map((h) => (
                  <option key={h.id} value={h.id}>
                    {h.label}
                  </option>
                ))}
              </select>
              <span className="text-xs text-fg-muted">
                Saved to <span className="font-mono">~/.config/kanban/config.toml</span>. Takes
                effect on the next session attach; running terminals keep their current process.
                Leave unset to fall back to the repo's{" "}
                <span className="font-mono">.kanban.toml</span> or the default.
              </span>
            </FormField>
            <FormField label="Worktrees directory">
              <FormInput
                mono
                type="text"
                className="disabled:opacity-50"
                value={worktreesRoot}
                placeholder={worktreesResolved || "~/.local/share/kanban/worktrees"}
                onChange={(e) => setWorktreesRoot(e.target.value)}
                disabled={settingsQ.isLoading || worktreesLocked}
                spellCheck={false}
              />
              <span className="text-xs text-fg-muted">
                Parent directory for new boards' worktrees. Leave empty to use the default. Supports{" "}
                <span className="font-mono">~</span> for your home directory. Existing boards keep
                their stored <span className="font-mono">worktree_root</span>.
                {worktreesLocked && (
                  <>
                    {" "}
                    Currently locked by <span className="font-mono">--worktrees-dir</span> or{" "}
                    <span className="font-mono">$KANBAN_WORKTREES_DIR</span>:{" "}
                    <span className="font-mono">{worktreesResolved}</span>.
                  </>
                )}
              </span>
            </FormField>
            <FormField label="Terminal position">
              <select
                className="rounded bg-surface px-2 py-1"
                value={orientation}
                onChange={(e) => setOrientation(e.target.value as TerminalOrientation)}
              >
                <option value="vertical">right</option>
                <option value="horizontal">bottom</option>
              </select>
            </FormField>
            <FormField label="Sign commits">
              <label className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={signCommits}
                  onChange={(e) => setSignCommits(e.target.checked)}
                  disabled={settingsQ.isLoading}
                />
                <span>Sign kanban's merge &amp; squash commits</span>
              </label>
              <span className="text-xs text-fg-muted">
                Off by default — kanban forces signing off so merges don't fail when the container
                has no key. When on, it defers to your gitconfig's{" "}
                <span className="font-mono">commit.gpgsign</span>, so mount your signing key and
                agent into the container. Saved to{" "}
                <span className="font-mono">~/.config/kanban/config.toml</span>.
              </span>
            </FormField>
          </>
        )}
        {tab === "appearance" && (
          <fieldset className="flex flex-col gap-2">
            <div className="flex flex-col gap-1">
              <span className="text-xs text-fg-muted">Theme</span>
              <ThemeModeToggle value={mode} onChange={setThemeMode} />
            </div>
            <label className="flex items-center gap-2">
              <input
                type="checkbox"
                checked={contrast === "high"}
                onChange={(e) => setContrast(e.target.checked ? "high" : ("normal" as Contrast))}
              />
              <span>High contrast</span>
            </label>
            <div className="flex flex-col gap-1">
              <span className="text-xs text-fg-muted">Accent</span>
              <AccentSwatches value={accent} onChange={setAccent} />
            </div>
          </fieldset>
        )}
        {tab === "shortcuts" && <KeybindingsSettings />}
        {tab === "developer" && devToolbarEnabled && (
          <fieldset className="flex flex-col gap-2">
            <label className="flex items-center gap-2">
              <input
                type="checkbox"
                checked={devToolbarPrefs.open}
                onChange={(e) => setDevToolbarPrefs({ ...devToolbarPrefs, open: e.target.checked })}
              />
              <span>Show developer toolbar</span>
            </label>
            <span className="text-xs text-fg-muted">
              Floating overlay of live frontend health metrics — FPS, JS heap, DOM / React Query
              activity, and SSE status. Use the toolbar's own controls to reposition it or choose
              which sections show.
            </span>
          </fieldset>
        )}
        <div className="mt-auto flex items-center justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose} disabled={busy}>
            cancel
          </Button>
          <Button
            type="submit"
            variant="secondary"
            size="lg"
            disabled={!dirty || busy}
            pending={updateMut.isPending}
            idleLabel="save"
            pendingLabel="saving…"
          />
        </div>
      </form>
      <footer className="border-t border-border px-4 py-2 font-mono text-[11px] text-fg-muted">
        {versionQ.data?.version ?? "…"}
      </footer>
    </Modal>
  );
}

function ThemeModeToggle({
  value,
  onChange,
}: {
  value: ThemeMode;
  onChange: (v: ThemeMode) => void;
}) {
  const opts: { v: ThemeMode; label: string }[] = [
    { v: "system", label: "System" },
    { v: "light", label: "Light" },
    { v: "dark", label: "Dark" },
  ];
  const activeIndex = Math.max(
    0,
    opts.findIndex((o) => o.v === value),
  );
  const refs = useRef<(HTMLButtonElement | null)[]>([]);
  const onKeyDown = (e: React.KeyboardEvent<HTMLButtonElement>) => {
    let next = activeIndex;
    if (e.key === "ArrowRight" || e.key === "ArrowDown") {
      next = (activeIndex + 1) % opts.length;
    } else if (e.key === "ArrowLeft" || e.key === "ArrowUp") {
      next = (activeIndex - 1 + opts.length) % opts.length;
    } else if (e.key === "Home") {
      next = 0;
    } else if (e.key === "End") {
      next = opts.length - 1;
    } else {
      return;
    }
    e.preventDefault();
    onChange(opts[next].v);
    refs.current[next]?.focus();
  };
  return (
    <div
      role="radiogroup"
      className="inline-flex w-fit overflow-hidden rounded border border-border"
    >
      {opts.map((o, i) => {
        const active = o.v === value;
        return (
          // biome-ignore lint/a11y/useSemanticElements: custom-styled radio toggle; <input type="radio"> can't carry the same visual treatment.
          <button
            key={o.v}
            ref={(el) => {
              refs.current[i] = el;
            }}
            type="button"
            role="radio"
            aria-checked={active}
            tabIndex={active ? 0 : -1}
            onClick={() => onChange(o.v)}
            onKeyDown={onKeyDown}
            className={`px-3 py-1 text-sm transition-colors duration-150 ${
              i > 0 ? "border-l border-border" : ""
            } ${active ? "bg-accent-700 text-white" : "bg-surface text-fg hover:bg-surface-2"}`}
          >
            {o.label}
          </button>
        );
      })}
    </div>
  );
}

function AccentSwatches({ value, onChange }: { value: Accent; onChange: (v: Accent) => void }) {
  return (
    <div className="flex flex-wrap gap-2">
      {ACCENTS.map((a) => {
        const active = a === value;
        return (
          <button
            key={a}
            type="button"
            data-accent-swatch={a}
            aria-label={ACCENT_LABELS[a]}
            aria-pressed={active}
            title={ACCENT_LABELS[a]}
            onClick={() => onChange(a)}
            data-accent={a}
            className={`h-7 w-7 rounded-full border border-border transition-transform duration-150 hover:scale-110 ${
              active ? "ring-2 ring-fg ring-offset-2 ring-offset-surface" : ""
            }`}
            style={{ backgroundColor: "var(--color-accent-700)" }}
          />
        );
      })}
    </div>
  );
}
