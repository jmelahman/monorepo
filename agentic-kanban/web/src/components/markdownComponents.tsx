import type { HTMLAttributes } from "react";
import type { Components } from "react-markdown";

// Inline Tailwind on each markdown element keeps us off the
// `@tailwindcss/typography` plugin (not installed here) while still
// producing usable hierarchy.
export const MARKDOWN_COMPONENTS: Components = {
  h1: ({ ...props }) => <h1 className="mb-3 mt-4 text-xl font-semibold" {...props} />,
  h2: ({ ...props }) => <h2 className="mb-2 mt-4 text-lg font-semibold" {...props} />,
  h3: ({ ...props }) => <h3 className="mb-2 mt-3 text-base font-semibold" {...props} />,
  h4: ({ ...props }) => <h4 className="mb-1 mt-2 text-sm font-semibold" {...props} />,
  p: ({ ...props }) => <p className="my-2" {...props} />,
  ul: ({ ...props }) => <ul className="my-2 list-disc pl-6" {...props} />,
  ol: ({ ...props }) => <ol className="my-2 list-decimal pl-6" {...props} />,
  li: ({ ...props }) => <li className="my-0.5" {...props} />,
  a: ({ ...props }) => (
    <a className="text-accent-500 hover:underline" target="_blank" rel="noreferrer" {...props} />
  ),
  blockquote: ({ ...props }) => (
    <blockquote className="my-2 border-l-2 border-border pl-3 italic text-fg-muted" {...props} />
  ),
  code: ({ className, children, ...props }: HTMLAttributes<HTMLElement>) => {
    const isBlock = typeof className === "string" && className.startsWith("language-");
    if (isBlock) {
      return (
        <code className={`${className} block`} {...props}>
          {children}
        </code>
      );
    }
    return (
      <code className="rounded bg-surface px-1 py-0.5 font-mono text-xs" {...props}>
        {children}
      </code>
    );
  },
  pre: ({ ...props }) => (
    <pre
      className="my-2 overflow-x-auto rounded bg-surface p-3 font-mono text-xs leading-snug"
      {...props}
    />
  ),
  table: ({ ...props }) => (
    <div className="my-2 overflow-x-auto">
      <table className="w-full border-collapse text-left text-xs" {...props} />
    </div>
  ),
  th: ({ ...props }) => (
    <th className="border-b border-border px-2 py-1 font-semibold" {...props} />
  ),
  td: ({ ...props }) => <td className="border-b border-border px-2 py-1" {...props} />,
  hr: ({ ...props }) => <hr className="my-4 border-border" {...props} />,
};
