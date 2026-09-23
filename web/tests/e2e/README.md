# Playwright E2E rules

Read before changing anything under `web/tests/e2e/`.

## Use Page Objects, not inline locators

Every UI surface gets a class that owns its locators and exposes high-level
actions. Specs call methods on that class — they don't construct selectors
themselves.

- One class per surface (`BoardPage`, `TicketCard`, `SessionPane`,
  `PtyTerminal`, …). Compose for nested surfaces:
  `boardPage.ticketCard(title).openTerminal()`.
- Writing the first spec for a surface that has no page object? Build the
  page object first, then the spec.
- A spec should read like a description of what the user does, not a tour
  of the DOM.

```typescript
// ✅ Good
await boardPage.goto(boardId);
await boardPage.ticketCard(ticket.title).click();
await boardPage.sessionPane.openTerminal();
await boardPage.sessionPane.expectStatus("idle");

// ❌ Bad
await page.goto("/");
await page
  .locator('[data-ticket-card="true"]')
  .filter({ hasText: ticket.title })
  .click();
await page.getByRole("button", { name: "terminal" }).click();
await expect(page.locator("[data-status-pill]")).toHaveText("idle");
```

The payoff: a selector change touches one file instead of every spec that
hit the surface, and code review becomes "do the user steps make sense?"
rather than "is this selector still right?"

## Assert with auto-retrying matchers, never with single-shot reads

`expect(locator).*` retries until it passes or the timeout fires.
`locator.getAttribute()`, `locator.textContent()`, `locator.count()`,
`page.evaluate()`, and the rest of the read APIs are one-shot snapshots —
they look at the DOM once and pass or fail on whatever happens to be there
at that instant. If the value can change from a React render, an effect,
or a microtask, that snapshot read is a flake waiting to happen.

| Asserting on | Use                                                  | Not                                       |
| ------------ | ---------------------------------------------------- | ----------------------------------------- |
| Attribute    | `expect(loc).toHaveAttribute(name, val)`             | `loc.getAttribute(name)` + `expect(...)`  |
| Class        | `expect(loc).toHaveClass(/re/)` / `.not.toHaveClass` | `evaluate(el => el.classList.contains())` |
| Text         | `expect(loc).toHaveText(...)` / `.toContainText`     | `loc.textContent()` + `expect(...)`       |
| Count        | `expect(loc).toHaveCount(n)`                         | `loc.count()` + `expect(...)`             |
| Visibility   | `expect(loc).toBeVisible()` / `.toBeHidden()`        | manual `isVisible()` checks               |
| Input value  | `expect(loc).toHaveValue(...)`                       | `loc.inputValue()` + `expect(...)`        |

```typescript
// ✅ retries until the attribute settles
await expect(card).toHaveAttribute("data-status", "idle");

// ❌ one-shot — flakes if the status updates a tick later
expect(await card.getAttribute("data-status")).toBe("idle");
```

The single-shot APIs are fine when you need the value for control flow
inside the spec (branching, logging) — just don't pin an assertion about
async state to them. If a standard matcher doesn't cover what you need,
reach for `expect.poll(...)` so the assertion still retries.
