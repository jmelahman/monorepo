import { useEffect, useState } from "react";
import { api, formatApiError, PUSH_EVENTS, type PushEvent } from "@/api/client";
import {
  currentPushSubscription,
  disablePush,
  enablePush,
  loadPushEvents,
  pushSupport,
  updatePushEvents,
} from "@/push";
import { useToast } from "@/toast";
import { Button } from "./Button";

const EVENT_LABELS: Record<PushEvent, string> = {
  awaiting_perm: "An agent is waiting for permission",
  finished: "An agent finished working",
  error: "A session failed or stopped unexpectedly",
};

const UNAVAILABLE: Record<Exclude<ReturnType<typeof pushSupport>, "ok">, string> = {
  insecure:
    "Notifications need a secure connection. Open kanban over HTTPS (or on localhost) to turn them on.",
  "ios-needs-install":
    "On iPhone and iPad, add kanban to the home screen first (Share → Add to Home Screen), then open it from there.",
  unsupported: "This browser doesn't support push notifications.",
};

// Per-device push settings. Unlike the rest of the settings form these apply
// immediately: subscribing involves a browser permission prompt, which can't
// be deferred to a save button.
export function NotificationSettings() {
  const { push } = useToast();
  const support = pushSupport();
  const [sub, setSub] = useState<PushSubscription | null>(null);
  const [events, setEvents] = useState<PushEvent[]>(loadPushEvents);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    currentPushSubscription()
      .then((s) => {
        if (!cancelled) setSub(s);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, []);

  if (support !== "ok") {
    return <p className="text-fg-muted">{UNAVAILABLE[support]}</p>;
  }

  const run = async (fn: () => Promise<void>) => {
    setBusy(true);
    try {
      await fn();
    } catch (err) {
      push("error", formatApiError(err));
    } finally {
      setBusy(false);
    }
  };

  const toggle = (on: boolean) =>
    run(async () => {
      if (on) {
        setSub(await enablePush(events));
      } else if (sub) {
        await disablePush(sub);
        setSub(null);
      }
    });

  const toggleEvent = (event: PushEvent, on: boolean) =>
    run(async () => {
      const next = PUSH_EVENTS.filter((e) => (e === event ? on : events.includes(e)));
      if (sub) await updatePushEvents(sub, next);
      setEvents(next);
    });

  return (
    <fieldset className="flex flex-col gap-3" disabled={busy}>
      <label className="flex items-center gap-2">
        <input type="checkbox" checked={sub != null} onChange={(e) => toggle(e.target.checked)} />
        <span>Send notifications to this device</span>
      </label>
      <span className="text-xs text-fg-muted">
        Arrive even when kanban is closed. Each browser or installed app is turned on separately.
      </span>
      <div className="flex flex-col gap-1">
        <span className="text-xs text-fg-muted">Notify me when</span>
        {PUSH_EVENTS.map((event) => (
          <label key={event} className="flex items-center gap-2">
            <input
              type="checkbox"
              checked={events.includes(event)}
              disabled={sub == null}
              onChange={(e) => toggleEvent(event, e.target.checked)}
            />
            <span className={sub == null ? "text-fg-muted" : ""}>{EVENT_LABELS[event]}</span>
          </label>
        ))}
      </div>
      <div>
        <Button
          type="button"
          variant="neutral"
          disabled={sub == null}
          onClick={() =>
            sub &&
            run(async () => {
              await api.testPush(sub.endpoint);
              push("success", "Test notification sent.");
            })
          }
        >
          send test notification
        </Button>
      </div>
    </fieldset>
  );
}
