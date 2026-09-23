/**
 * Just enough DOM helper to avoid string-concatenating HTML.
 *
 * There is no framework here on purpose: the whole game is one grid, one
 * keyboard, a relic row and a shop. A framework would add weight without
 * solving anything, and this file is the entire cost of not having one.
 */

type Attrs = Record<string, string | number | boolean | undefined | EventListener>
type Child = Node | string | null | undefined | false

export function h<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  attrs: Attrs = {},
  ...children: Child[]
): HTMLElementTagNameMap[K] {
  const node = document.createElement(tag)

  for (const [key, value] of Object.entries(attrs)) {
    if (value === undefined || value === false) continue
    if (key.startsWith("on") && typeof value === "function") {
      node.addEventListener(key.slice(2).toLowerCase(), value)
    } else if (key === "class") {
      node.className = String(value)
    } else if (value === true) {
      node.setAttribute(key, "")
    } else {
      node.setAttribute(key, String(value))
    }
  }

  for (const child of children) {
    if (child === null || child === undefined || child === false) continue
    node.append(typeof child === "string" ? document.createTextNode(child) : child)
  }
  return node
}

export function clear(node: HTMLElement): HTMLElement {
  node.replaceChildren()
  return node
}

export const wait = (ms: number): Promise<void> => new Promise((resolve) => setTimeout(resolve, ms))
