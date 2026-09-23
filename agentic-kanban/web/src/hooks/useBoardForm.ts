import { useCallback, useState } from "react";

export type BoardFormFields = {
  name: string;
  repo: string;
  mount: string;
  worktreeRoot: string;
  base: string;
  branchPrefix: string;
  gitAuthorName: string;
  gitAuthorEmail: string;
};

const EMPTY: BoardFormFields = {
  name: "",
  repo: "",
  mount: "",
  worktreeRoot: "",
  base: "main",
  branchPrefix: "",
  gitAuthorName: "",
  gitAuthorEmail: "",
};

// useBoardForm owns the field state shared by CreateBoardForm and BoardSettings.
// Consumers read whichever fields they render; unused ones stay at their EMPTY
// defaults. `reset` re-seeds the whole form, e.g. when an edit modal re-opens
// against a different board.
export function useBoardForm(initial?: Partial<BoardFormFields>) {
  const [fields, setFields] = useState<BoardFormFields>({ ...EMPTY, ...initial });

  const update = useCallback(
    <K extends keyof BoardFormFields>(key: K, value: BoardFormFields[K]) => {
      setFields((prev) => ({ ...prev, [key]: value }));
    },
    [],
  );

  const reset = useCallback((next?: Partial<BoardFormFields>) => {
    setFields({ ...EMPTY, ...next });
  }, []);

  const hasRepo = fields.repo.trim() !== "";
  const hasMount = fields.mount.trim() !== "";

  return { fields, update, reset, hasRepo, hasMount };
}
