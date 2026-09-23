export type ActionId = string;

export interface Binding {
  ctrl: boolean;
  alt: boolean;
  shift: boolean;
  meta: boolean;
  // Normalized key: uppercase letter for A–Z, otherwise the KeyboardEvent.key
  // value (e.g. "PageDown", "ArrowLeft", "Enter").
  key: string;
}

export interface Action {
  id: ActionId;
  label: string;
  group: string;
  defaultBinding: Binding | null;
}
