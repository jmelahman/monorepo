import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api, type Step } from "@/api/client";
import { Button, Dialog, ErrorText, Slider } from "@/components/ui";

// CompleteDialog records a finished step with optional mastery and pleasure
// ratings (behavioral activation). Skipping the ratings is always fine.
export default function CompleteDialog({
  step,
  onClose,
}: {
  step: Step | null;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const [mastery, setMastery] = useState<number | null>(null);
  const [pleasure, setPleasure] = useState<number | null>(null);
  const done = useMutation({
    mutationFn: (rated: boolean) =>
      api.completeStep(
        (step as Step).id,
        rated ? (mastery ?? undefined) : undefined,
        rated ? (pleasure ?? undefined) : undefined,
      ),
    onSuccess: () => {
      qc.invalidateQueries();
      setMastery(null);
      setPleasure(null);
      onClose();
    },
  });
  return (
    <Dialog open={step !== null} onClose={onClose} title="Nice. That counts.">
      <p className="mb-4 text-sm text-fg-muted">
        How did “{step?.title}” feel? Rating helps you spot what lifts your mood.
      </p>
      <div className="space-y-4">
        <Slider
          label="Mastery (sense of accomplishment)"
          value={mastery}
          onChange={setMastery}
          low="none"
          high="a lot"
        />
        <Slider
          label="Pleasure (enjoyment)"
          value={pleasure}
          onChange={setPleasure}
          low="none"
          high="a lot"
        />
        <ErrorText error={done.error} />
        <div className="flex gap-2">
          <Button onClick={() => done.mutate(true)} disabled={done.isPending} className="flex-1">
            Save
          </Button>
          <Button variant="soft" onClick={() => done.mutate(false)} disabled={done.isPending}>
            Skip ratings
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
