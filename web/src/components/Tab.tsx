export function Tab({
  active,
  onClick,
  label,
}: {
  active: boolean;
  onClick: () => void;
  label: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex items-center px-2 py-2 -mb-px border-b-2 transition-colors duration-150 sm:px-3 ${active ? "border-accent-500 text-fg" : "border-transparent text-fg-muted hover:text-fg"}`}
    >
      {label}
    </button>
  );
}
