/**
 * Just enough of a browser for `src/ui/views.ts` to run inside QuickJS.
 *
 * The desktop build draws the web build's own views rather than a second set
 * written for Godot. They are pure `state → element` functions, and everything
 * they say, show and hide is decided there, down to which card carries which
 * tip and which button is disabled. A Godot copy of the same decisions would be
 * 3,800 lines to keep in step with the first by hand. So the views run here,
 * against this, and what they build is handed to Godot as a JSON tree for
 * `game/render.gd` to draw. Godot owns how it looks; the views own what it is.
 *
 * The surface is what the views and the modules under them actually touch, and
 * no more: `createElement`, `createElementNS`, `createTextNode`, attributes,
 * `className`/`classList`/`dataset`/`style`, `append`/`replaceChildren`,
 * `textContent`, and a `querySelector` that understands the two selectors
 * `fillCoach` asks it (`.coach`, `.coach-text`). Listeners are kept on the node
 * and given ids when the tree is serialized, which is how a click in Godot
 * finds its way back to the closure the view wrote.
 *
 * Also here, because they are the same kind of thing: `localStorage` (a
 * dictionary Godot persists to `user://`), `navigator.languages`, `Intl` for
 * the four catalogs, and the two build constants Vite would define.
 *
 * Imported first by `shell.ts`, so it has run before any module that reads a
 * global at load. `en.ts` builds its `Intl.PluralRules` at import time.
 */

type Listener = (event: FakeEvent) => void

/** What a handler is handed. The views' handlers ignore it; the shape is for safety. */
export type FakeEvent = {
  type: string
  target: FakeElement
  currentTarget: FakeElement
  preventDefault(): void
  stopPropagation(): void
}

export class FakeText {
  parentNode: FakeElement | null = null
  constructor(public data: string) {}
  get textContent(): string {
    return this.data
  }
  set textContent(value: string) {
    this.data = value
  }
  remove(): void {
    this.parentNode?.removeChild(this)
  }
}

type Child = FakeElement | FakeText

export class FakeElement {
  readonly attributes = new Map<string, string>()
  readonly childNodes: Child[] = []
  readonly listeners = new Map<string, Listener[]>()
  readonly style = new FakeStyle()
  parentNode: FakeElement | null = null
  /** `hidden`, `disabled`, `href`... properties the views set directly. */
  readonly props: Record<string, unknown> = {}

  constructor(
    readonly tagName: string,
    readonly svg = false,
  ) {}

  get parentElement(): FakeElement | null {
    return this.parentNode
  }

  get children(): FakeElement[] {
    return this.childNodes.filter((child): child is FakeElement => child instanceof FakeElement)
  }

  get firstElementChild(): FakeElement | null {
    return this.children[0] ?? null
  }

  get className(): string {
    return this.attributes.get("class") ?? ""
  }
  set className(value: string) {
    this.attributes.set("class", value)
  }

  get classList(): FakeClassList {
    return new FakeClassList(this)
  }

  /** Live, as the real one is: a write lands on the `data-*` attribute. */
  get dataset(): Record<string, string | undefined> {
    return new Proxy({} as Record<string, string | undefined>, {
      get: (_, key) => this.attributes.get(`data-${kebab(String(key))}`),
      set: (_, key, value) => {
        this.attributes.set(`data-${kebab(String(key))}`, String(value))
        return true
      },
      deleteProperty: (_, key) => this.attributes.delete(`data-${kebab(String(key))}`),
    })
  }

  get id(): string {
    return this.attributes.get("id") ?? ""
  }
  set id(value: string) {
    this.attributes.set("id", value)
  }

  get textContent(): string {
    return this.childNodes.map((child) => child.textContent).join("")
  }
  set textContent(value: string) {
    this.replaceChildren(value)
  }

  setAttribute(name: string, value: string): void {
    this.attributes.set(name, String(value))
  }
  getAttribute(name: string): string | null {
    return this.attributes.get(name) ?? null
  }
  hasAttribute(name: string): boolean {
    return this.attributes.has(name)
  }
  removeAttribute(name: string): void {
    this.attributes.delete(name)
  }

  addEventListener(type: string, listener: Listener): void {
    const list = this.listeners.get(type) ?? []
    list.push(listener)
    this.listeners.set(type, list)
  }
  removeEventListener(type: string, listener: Listener): void {
    const list = this.listeners.get(type)
    if (list)
      this.listeners.set(
        type,
        list.filter((entry) => entry !== listener),
      )
  }

  append(...nodes: (Child | string)[]): void {
    for (const node of nodes) this.adopt(typeof node === "string" ? new FakeText(node) : node)
  }
  appendChild<T extends Child>(node: T): T {
    this.adopt(node)
    return node
  }
  prepend(...nodes: (Child | string)[]): void {
    const adopted = nodes.map((node) => (typeof node === "string" ? new FakeText(node) : node))
    for (const node of adopted.reverse()) {
      node.parentNode?.removeChild(node)
      node.parentNode = this
      this.childNodes.unshift(node)
    }
  }
  replaceChildren(...nodes: (Child | string)[]): void {
    for (const child of this.childNodes) child.parentNode = null
    this.childNodes.length = 0
    this.append(...nodes)
  }
  removeChild<T extends Child>(node: T): T {
    const at = this.childNodes.indexOf(node)
    if (at >= 0) this.childNodes.splice(at, 1)
    node.parentNode = null
    return node
  }
  remove(): void {
    this.parentNode?.removeChild(this)
  }

  private adopt(node: Child): void {
    node.parentNode?.removeChild(node)
    node.parentNode = this
    this.childNodes.push(node)
  }

  querySelector(selector: string): FakeElement | null {
    return this.querySelectorAll(selector)[0] ?? null
  }

  /**
   * Descendants matching one compound selector: classes, a tag, and
   * `[attr="value"]`. Descendant combinators are matched right to left against
   * ancestors, which covers every selector this build asks.
   */
  querySelectorAll(selector: string): FakeElement[] {
    const steps = selector.trim().split(/\s+/)
    const found: FakeElement[] = []
    const walk = (node: FakeElement) => {
      for (const child of node.children) {
        if (matchesChain(child, steps, this)) found.push(child)
        walk(child)
      }
    }
    walk(this)
    return found
  }

  matches(selector: string): boolean {
    return matchesChain(this, selector.trim().split(/\s+/), null)
  }

  closest(selector: string): FakeElement | null {
    for (let node: FakeElement | null = this; node; node = node.parentNode) {
      if (node.matches(selector)) return node
    }
    return null
  }

  // Focus has no meaning here: Godot keeps its own.
  focus(): void {}
  blur(): void {}
}

class FakeClassList {
  constructor(private readonly el: FakeElement) {}
  private get names(): string[] {
    return this.el.className.split(/\s+/).filter(Boolean)
  }
  contains(name: string): boolean {
    return this.names.includes(name)
  }
  add(...names: string[]): void {
    const now = this.names
    for (const name of names) if (!now.includes(name)) now.push(name)
    this.el.className = now.join(" ")
  }
  remove(...names: string[]): void {
    this.el.className = this.names.filter((name) => !names.includes(name)).join(" ")
  }
  toggle(name: string, force?: boolean): boolean {
    const on = force ?? !this.contains(name)
    if (on) this.add(name)
    else this.remove(name)
    return on
  }
  [Symbol.iterator](): Iterator<string> {
    return this.names[Symbol.iterator]()
  }
}

/** `style.setProperty` and plain assignment both land in `values`. */
class FakeStyle {
  readonly values: Record<string, string> = {}
  constructor() {
    // biome-ignore lint/correctness/noConstructorReturn: a Proxy is the point.
    return new Proxy(this, {
      get: (target, key) =>
        key in target ? target[key as keyof FakeStyle] : target.values[kebab(String(key))],
      set: (target, key, value) => {
        target.values[kebab(String(key))] = String(value)
        return true
      },
    })
  }
  setProperty(name: string, value: string): void {
    this.values[name] = String(value)
  }
  removeProperty(name: string): void {
    delete this.values[name]
  }
  getPropertyValue(name: string): string {
    return this.values[name] ?? ""
  }
}

const kebab = (name: string): string => name.replace(/[A-Z]/g, (c) => `-${c.toLowerCase()}`)

function matchesOne(el: FakeElement, compound: string): boolean {
  const attrs = [...compound.matchAll(/\[([\w-]+)(?:="?([^"\]]*)"?)?\]/g)]
  const bare = compound.replace(/\[[^\]]*\]/g, "")
  const [tag, ...classes] = bare.split(".")
  if (tag && tag !== "*" && tag.toLowerCase() !== el.tagName.toLowerCase()) return false
  const has = el.classList
  if (!classes.every((name) => has.contains(name))) return false
  return attrs.every(([, name = "", value]) =>
    value === undefined ? el.hasAttribute(name) : el.getAttribute(name) === value,
  )
}

function matchesChain(el: FakeElement, steps: string[], scope: FakeElement | null): boolean {
  const last = steps[steps.length - 1]
  if (last === undefined || !matchesOne(el, last)) return false
  let rest = steps.length - 2
  for (let node = el.parentNode; node && node !== scope && rest >= 0; node = node.parentNode) {
    const step = steps[rest]
    if (step !== undefined && matchesOne(node, step)) rest--
  }
  return rest < 0
}

/* --------------------------------------------------------------- document */

const root = new FakeElement("html")

const fakeDocument = {
  documentElement: root,
  createElement: (tag: string) => new FakeElement(tag),
  createElementNS: (_ns: string, tag: string) => new FakeElement(tag, true),
  createTextNode: (data: string) => new FakeText(data),
  querySelectorAll: () => [],
  querySelector: () => null,
  addEventListener: () => {},
  get activeElement() {
    return null
  },
}

/* ----------------------------------------------------------------- storage */

/**
 * The web build's `localStorage`, held in memory and written out by Godot.
 *
 * Every module that remembers something (`meta.ts`, `lang/index.ts`, the
 * shell) keeps its web key and its web encoding, so the record, the settings
 * and the run all live where the web build put them and read the same way. What
 * changes is who owns the disk: `dirty` is raised on every write and Godot, which
 * asks after each call, saves the whole store to `user://` when it is.
 */
export const store = {
  items: new Map<string, string>(),
  dirty: false,
}

const fakeStorage = {
  getItem: (key: string): string | null => store.items.get(key) ?? null,
  setItem(key: string, value: string): void {
    store.items.set(key, String(value))
    store.dirty = true
  },
  removeItem(key: string): void {
    if (store.items.delete(key)) store.dirty = true
  },
  clear(): void {
    store.items.clear()
    store.dirty = true
  },
  key: (index: number): string | null => [...store.items.keys()][index] ?? null,
  get length() {
    return store.items.size
  },
}

/* -------------------------------------------------------------------- Intl */

/**
 * CLDR's cardinal rules for the four catalogs, which is all `pluralizer` asks.
 *
 * QuickJS has no `Intl`. Each catalog names its locale once and asks
 * `rules.select(n)` for every agreeing sentence, so a rule wrong here is a
 * sentence wrong in one language and right in the other three, which is
 * exactly the bug nobody writing in the others would look for. These are the
 * CLDR 45 rules, spelled out rather than approximated: French puts 0 and 1.5 in
 * `one`, and Spanish and French both have a `many` for exact millions, which
 * the catalogs let fall through to `other` and which is here anyway so that one
 * a translator adds later is honoured.
 */
const PLURALS: Record<string, (n: number) => Intl.LDMLPluralRule> = {
  en: (n) => (n === 1 ? "one" : "other"),
  de: (n) => (n === 1 ? "one" : "other"),
  es: (n) => (n === 1 ? "one" : n !== 0 && Number.isInteger(n) && n % 1e6 === 0 ? "many" : "other"),
  fr: (n) => {
    const i = Math.trunc(Math.abs(n))
    if (i === 0 || i === 1) return "one"
    return Number.isInteger(n) && n % 1e6 === 0 ? "many" : "other"
  },
}

const primary = (locale: string): string => locale.toLowerCase().split("-")[0] ?? "en"

class PluralRules {
  private readonly rule: (n: number) => Intl.LDMLPluralRule
  constructor(locale: string) {
    const rule = PLURALS[primary(locale)]
    if (!rule) throw new Error(`dom-shim: no plural rules for ${locale}`)
    this.rule = rule
  }
  select(count: number): Intl.LDMLPluralRule {
    return this.rule(count)
  }
}

/** The conjunction lists `creditsView` builds for its thank-you line. */
const AND: Record<string, string> = { en: "and", es: "y", fr: "et", de: "und" }

class ListFormat {
  private readonly and: string
  constructor(locale: string) {
    this.and = AND[primary(locale)] ?? "and"
  }
  format(items: Iterable<string>): string {
    const list = [...items]
    if (list.length < 3) return list.join(` ${this.and} `)
    // The Oxford comma is English's alone.
    const comma = this.and === "and" ? "," : ""
    return `${list.slice(0, -1).join(", ")}${comma} ${this.and} ${list[list.length - 1]}`
  }
}

/* ----------------------------------------------------------------- install */

const scope = globalThis as Record<string, unknown>
scope.document = fakeDocument
scope.localStorage = fakeStorage
scope.navigator = { languages: [] as string[] }
scope.Intl = { PluralRules, ListFormat }
scope.window = scope
// `reportUrl` asks where it is served to tell the APK from the site; the
// desktop build is neither, and reads as the browser, which is the form's
// catch-all.
scope.location = { protocol: "app:", hostname: "desktop", href: "app://desktop/" }
scope.URLSearchParams = class {
  constructor(private readonly params: Record<string, string>) {}
  toString(): string {
    return Object.entries(this.params)
      .map(([key, value]) => `${encodeURIComponent(key)}=${encodeURIComponent(value)}`)
      .join("&")
  }
}
// Vite's `define`s; `scripts/bundle.sh` passes the real ones as a banner.
scope.__BUILD_VERSION__ ??= "0.0.0"
scope.__BUILD_COMMIT__ ??= ""

/* --------------------------------------------------------------- serialize */

/**
 * One node of the tree Godot draws. Short keys, because a round screen is a few
 * hundred of these and the text is parsed on every render.
 *
 * `t` tag, `c` class, `a` every other attribute (`data-*` included), `s` inline
 * style, `k` children, `x` a text node's text, `on` the id of its click handler.
 * An `<svg>` arrives whole as `svg`, its markup, since Godot draws it as one
 * image rather than as nodes.
 */
export type Node =
  | string
  | {
      t: string
      c?: string
      a?: Record<string, string>
      s?: Record<string, string>
      k?: Node[]
      on?: number
      svg?: string
    }

/** Every click handler in the tree last serialized, by the id it was given. */
let handlers: Listener[] = []

export function serialize(el: FakeElement): Node {
  handlers = []
  return walk(el)
}

function walk(el: FakeElement): Node {
  if (el.svg) return { t: "svg", c: el.className, svg: svgMarkup(el) }
  const out: Exclude<Node, string> = { t: el.tagName.toLowerCase() }
  const attrs: Record<string, string> = {}
  for (const [name, value] of el.attributes) {
    if (name === "class") out.c = value
    else attrs[name] = value
  }
  if (Object.keys(attrs).length > 0) out.a = attrs
  if (Object.keys(el.style.values).length > 0) out.s = { ...el.style.values }
  const click = el.listeners.get("click")?.[0]
  if (click) {
    out.on = handlers.length
    handlers.push(click)
  }
  const kids: Node[] = []
  for (const child of el.childNodes) {
    if (child instanceof FakeText) {
      if (child.data !== "") kids.push(child.data)
    } else {
      kids.push(walk(child))
    }
  }
  if (kids.length > 0) out.k = kids
  return out
}

const escapeXml = (text: string): string =>
  text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/"/g, "&quot;")

function svgMarkup(el: FakeElement): string {
  const attrs = [...el.attributes]
    .filter(([name]) => name !== "class" && name !== "aria-hidden")
    .map(([name, value]) => ` ${name}="${escapeXml(value)}"`)
    .join("")
  const xmlns = el.tagName === "svg" ? ' xmlns="http://www.w3.org/2000/svg"' : ""
  const inner = el.childNodes
    .map((child) => (child instanceof FakeText ? escapeXml(child.data) : svgMarkup(child)))
    .join("")
  return `<${el.tagName}${xmlns}${attrs}>${inner}</${el.tagName}>`
}

/** Runs the handler Godot says was clicked. False if the id is stale. */
export function click(id: number, target: FakeElement): boolean {
  const handler = handlers[id]
  if (!handler) return false
  handler({
    type: "click",
    target,
    currentTarget: target,
    preventDefault() {},
    stopPropagation() {},
  })
  return true
}
