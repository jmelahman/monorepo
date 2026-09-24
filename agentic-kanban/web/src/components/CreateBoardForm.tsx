import { useMutation } from "@tanstack/react-query";
import { api, type Board } from "@/api/client";
import { useBoardForm } from "@/hooks/useBoardForm";
import { Button } from "./Button";
import { FormField, FormInput } from "./FormField";
import { Modal } from "./Modal";

export function CreateBoardModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (b: Board) => void;
}) {
  const { fields, update, reset, hasRepo } = useBoardForm();

  const createMut = useMutation({
    mutationFn: () =>
      api.createBoard({
        name: fields.name,
        repo_path: fields.repo.trim(),
        project_dir: fields.projectDir.trim(),
      }),
    onSuccess: (board) => {
      onCreated(board);
      onClose();
      reset();
    },
  });

  const handleClose = () => {
    if (createMut.isPending) return;
    onClose();
  };

  const canSubmit = fields.name.trim() !== "" && hasRepo;

  return (
    <Modal open={open} onClose={handleClose} title="New board" busy={createMut.isPending}>
      <form
        className="flex flex-col gap-3 p-4 text-sm"
        onSubmit={(e) => {
          e.preventDefault();
          if (!canSubmit) return;
          createMut.mutate();
        }}
      >
        <FormField label="Name">
          <FormInput
            value={fields.name}
            onChange={(e) => update("name", e.target.value)}
            required
            autoFocus
          />
        </FormField>
        <FormField
          label="Repository path"
          hint="Host path to the git repo this board tracks. Mount path defaults to this and can be customized in board settings."
        >
          <FormInput
            mono
            placeholder="/host/path/to/repo"
            value={fields.repo}
            onChange={(e) => update("repo", e.target.value)}
            required
          />
        </FormField>
        <FormField
          label="Project directory"
          hint="Optional. For a monorepo: the repo-relative subdirectory the agent works from, e.g. services/api. The whole repo is still checked out and mounted."
        >
          <FormInput
            mono
            placeholder="services/api"
            value={fields.projectDir}
            onChange={(e) => update("projectDir", e.target.value)}
          />
        </FormField>
        <div className="mt-2 flex items-center justify-end gap-2">
          <Button
            type="button"
            variant="ghost"
            disabled={createMut.isPending}
            onClick={handleClose}
          >
            cancel
          </Button>
          <Button
            type="submit"
            variant="primary"
            size="lg"
            disabled={!canSubmit}
            pending={createMut.isPending}
            idleLabel="create"
            pendingLabel="creating…"
          />
        </div>
      </form>
    </Modal>
  );
}
