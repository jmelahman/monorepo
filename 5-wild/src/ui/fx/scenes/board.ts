/**
 * The board on the table: letters dropping in, backspace, a refused row, the
 * board dealt at the start of a round. Owned by phase 3 of
 * `plans/ui-overhaul.md`.
 *
 * Every hook here is called on the phone too, and must return at once there:
 * check `juiced()` first.
 */

/** A letter just landed in `tile` (after `patchDraft` drew it). */
export function typed(_tile: HTMLElement): void {}

/** A letter just left the row being typed. */
export function erased(_row: HTMLElement): void {}

/** The row being typed was refused (after its `rejected` class went on). */
export function rejected(_row: HTMLElement): void {}

/** A round screen just arrived from somewhere that was not a round. */
export function dealt(_screen: HTMLElement): void {}
