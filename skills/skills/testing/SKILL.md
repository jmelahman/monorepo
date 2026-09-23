---
name: testing
description: >-
  Rules for tests that fail only when real behavior breaks: coverage, test doubles, naming,
  assertions, and testability. Use before writing or changing a test, when reviewing tests, when
  deciding what to test or whether to use a real object, fake, or mock, when a test is flaky,
  brittle, or breaks on every refactor, or when production code is hard to test.
---

# Testing

Distilled from Google's Testing on the Toilet. The unifying principle: **a test
should fail when — and only when — a behavior the user cares about breaks, and
its failure should be actionable from the test name and message alone.**
Everything below serves that.

Scope: apply these rules to tests and code you are writing or changing. In an
existing suite, established local convention wins — table-driven tests in Go,
pytest fixtures, the house assertion library are idiom, not violations. Use
these rules to break ties and shape new code; flag pre-existing violations
rather than rewriting them unasked. The exception is a convention that is
itself the defect — a mock-everything suite, sleep-based waits: don't silently
reproduce it; surface the conflict and let the owner decide. Code snippets
throughout are illustrative — translate them to the local language and
framework.

[references/patterns.md](references/patterns.md) has worked Bad/Good pairs for
the rules most often misapplied. [references/INDEX.md](references/INDEX.md)
maps every rule to its source post; when you need the full argument behind a
rule, read the local copy under `references/episodes/` if present,
otherwise fetch the post's URL. (Authors: `scripts/fetch_episodes.py` builds
the local copies.)

## The loop

For each test you write or review:

1. **Name the risk** — the concrete bug this test exists to catch.
2. **Pick the cheapest layer** (SMURF, below) that can catch that bug;
   escalate to a larger test only by stating what the cheaper layer misses.
3. **Choose doubles by the ladder**: real → fake → mock (below).
4. **Write it against the public API**, asserting outcomes.
5. **Prove it can fail**: run it against deliberately broken code, or at
   minimum name the behavior bug that would fail it. A test with no nameable
   failing bug is a change-detector — don't ship it.

If the only test you can write is a tautology, a mock-script of the
implementation, or wiring with no logic to exercise: don't add it. Say so,
and report the missing seam — or the layer that should own the risk —
instead. An honest gap beats a test that can't fail.

## What to test

- **Test behaviors, not methods.** One test per behavior; a method may have
  many behaviors, a behavior may span methods. Smell: a test named after a
  method that keeps accreting assertions.
- **Test through the public API; treat internals as implementation details.**
  Helper classes used by one caller usually don't need their own tests — cover
  them through the API that uses them, and restrict their visibility. Tests
  written this way survive refactoring (only setup should change when the
  implementation does).
- **Assert outcomes (state), not interactions.** A test that mirrors the
  production call graph with mock verifications is a *change-detector*: it
  fails on every refactor and passes on broken logic — negative value; rewrite
  or delete it. Verify interactions only when the interaction *is* the
  contract: side effects (exactly one email sent), call count/order that
  matters (caching, deadlock avoidance), or a presenter→view boundary.
- **Coverage is necessary, not sufficient.** Statement coverage misses the
  invisible `else` on every `if`, short-circuited operands, untested inputs
  (`b / c` never run with `c == 0`), and code that *should* exist but doesn't.
  Cover branches and the boundary values of each sub-condition — but don't
  enumerate combinatorics; extract sub-expressions and test them independently
  (for k sub-conditions: (k+1) + 2·k tests instead of 3^k).
- **Balance the suite by trade-offs, not dogma** (SMURF): Speed,
  Maintainability, Utilization (resource cost), Reliability, Fidelity. Unit
  tests win S/M/U/R; a few integration and end-to-end tests buy the F.
- **End-to-end tests: one per critical user journey** plus one per important
  class of error. Verify overall behavior, not exact messages or layouts;
  budget real maintenance time for them; keep their data ephemeral.
- **Test UI logic through the component, not the handler** ("front door
  first") — rendering the component and dispatching real events catches bugs
  (a disabled button) that calling the handler directly cannot.
- **Multiple implementations of one interface share one contract test**:
  an abstract test class with a factory hook per implementation.
- **Heuristic/approximate code**: keep a must-pass set of easy cases plus a
  thresholded set of hard cases (e.g. "≥50% of these decode") so you can
  include tough inputs and watch quality drift instead of avoiding them.
- **Start from risk, not ritual.** Ask what can actually go wrong (data loss,
  outage) and test that first; sometimes the cheapest effective mitigation
  isn't an automated test at all.

## Test doubles

- **Prefer, in order: real implementation → fake → mock.** Use the real thing
  when it's fast, deterministic, and easy to construct. Use a fake (in-memory
  implementation) when it isn't. Reach for a mock only when neither exists,
  or to force hard-to-trigger paths (timeouts, errors).
- Vocabulary: **dummy** (fills a parameter), **stub** (canned answers),
  **spy** (records calls, asserted as state), **mock** (verifies
  expectations), **fake** (working lightweight implementation).
- **Fakes**: for a real dependency, use the fake its owner maintains, tested
  against the same contract as the real implementation (run one test suite
  over both) and written at the lowest level possible (fake the database, not
  every class that touches it). When you must write your own for a narrow
  role, keep it minimal — a single-use fake beats a general-purpose replica.
- **Don't mock types you don't own.** Wrap the third-party API in your own
  thin type; mock the wrapper, and test the wrapper against the real library.
  Mocks of foreign APIs silently drift from reality on every upgrade.
- **Don't mock what you can exercise.** If code depends on a service
  contract, run the call against a fake or hermetic local server so the
  contract itself is checked; a mocked response proves nothing about validity.
- **Overuse smells**: mocking more than one or two collaborators; a mock
  scripted with several `when(...)` lines; mentally stepping through
  production code to understand the test. Each `when` leaks implementation
  into the test and drifts from the real behavior. The fix is usually a
  missing seam: extract a narrower interface around the cluster and fake
  that.
- **Verify only state-changing calls** (`sendEmail`, `saveRecord`) — never
  that a getter was called; use getters/stubs to *arrange* conditions and
  assert on results instead.
- **Verify only the arguments relevant to the behavior under test**; use
  matchers (`contains(...)`, `any()`) for the rest, and cover other arguments
  in their own tests.
- **Inject dependencies; don't reach out.** Singletons, static calls, and
  patched globals make code untestable and tests order-dependent. Pass
  collaborators through the constructor (or parameterize the function —
  `Foo(path, checker=os.path.exists)`) instead of monkey-patching, which
  breaks anything else using the global.

## Structure and readability

- **DAMP over DRY.** Tests don't have tests — they must be obviously correct
  on inspection. Prefer descriptive duplication to clever reuse; keep DRY for
  value-object helpers that remove noise without hiding meaning.
- **No logic in tests**: no loops, conditionals, or computed expectations.
  State inputs and expected outputs literally (`"http://plus.google.com//u/0/photos"`
  would have exposed the double-slash bug the concatenation hid). Nontrivial
  test utilities get their own tests.
- **Complete and concise**: everything needed to understand the test is in
  its body; nothing else is. Pass only the relevant detail to helpers
  (`_create_account(BALANCE)`), hide the noise (settings, addresses) inside
  them, and never let a test depend on a helper's default values.
- **Keep cause and effect adjacent.** Don't build state in a distant
  `setUp`/fixture and assert on it 200 lines later — each test arranges what
  it asserts.
- **One scenario per test.** Smell: asserting, then calling the system under
  test again. Focused tests isolate failures, keep setup simple, and turn the
  test list into a behavior catalog.
- **Test data builders** beat parameter-list helpers: `newCompany().setType(PRIVATE).build()`
  with required fields defaulted, each test setting only what it cares about.
- **Choose values that can't pass by accident**: non-default values (not `0`,
  `""`, or the first enum), a different value for each input so swapped
  arguments fail, plus boundaries and empty/null cases.
- **Keep data in the test.** Table-driven tests over heterogeneous scenarios
  grow superlinearly, hide which case failed, and develop their own bugs;
  split them into named tests per code path (parameterized tests are fine for
  genuinely uniform inputs).
- **Names carry scenario and expected outcome**:
  `isUserLockedOut_lockOutUserAfterThreeInvalidLoginAttempts`. Reading the
  test list should enumerate the class's responsibilities; a missing behavior
  is then visible as a missing name.
- **Refactor tests in the red**: before refactoring test code, break the
  production code and confirm the assertions still fail; refactor production
  code only in the green. (Then fix the production code back!)

## Assertions and failure messages

- **A failure must be actionable without a rerun**: name + message should
  localize the bug. Use expressive assertion APIs/matchers
  (`assertThat(x).contains(y)`, `EXPECT_THAT(v, UnorderedElementsAre(...))`)
  that print expected *and* actual in domain terms.
- **Narrow assertions**: assert the fields the behavior touches, not
  whole-object equality — broad equality makes every unrelated schema change
  break every test. At most one broad "everything" test per object; one
  screenshot test for layout, DOM assertions for behaviors.
- **Brittleness is an API gap**: if tests keep depending on hash order, exact
  logs, or golden diffs, the fix is a more expressive matcher (or filing for
  one), not more careful tiptoeing.
- **Messages state expected behavior** — "BLUE and VIOLET *should* be
  indistinguishable" — so the reader needn't invert an `assertFalse` (the
  Stroop effect). Prefer EXPECT-style (continue-on-failure) assertions to see
  all failures per run; ASSERT only when continuing makes no sense.
- **Floating point**: never assert exact equality; use approximate
  comparisons (`EXPECT_DOUBLE_EQ`, delta-based `assertEquals`).

## Flakiness and nondeterminism

- **Sleeping is not synchronization.** Replace `sleep` with explicit
  coordination — events, latches, notifications — with generous timeouts that
  only matter on failure. Test the single-threaded logic separately from the
  threading behavior.
- **Time is an input.** Code that calls `now()` internally has a hidden
  random input; inject the time (or a clock) and keep a thin deprecated
  wrapper for callers. Use fake/mock timers to test scheduled callbacks
  instantly.
- **Own your resources**: unique temp paths per test, no shared files,
  databases, or assumed preconditions; check or create what you rely on.
- **Races need detectors.** Ordinary tests rarely expose data races; run
  ThreadSanitizer (`-fsanitize=thread`, Go `-race`) in CI and treat every
  reported race as a real bug — there are no benign ones.

## Design for testability

- **Functional core, imperative shell**: pure logic that only touches its
  arguments, wrapped by a thin shell that does I/O. The core tests trivially;
  the shell stays too simple to hide bugs.
- **Construct with collaborators, call with work**: lifetime dependencies go
  in the constructor; per-operation data goes in method parameters.
- **Long method → extract methods** (comments marking sections are the hint);
  **private logic worth testing → extract a class/interface** and inject it —
  never `friend`/reflection back-doors into privates.
- **Wrap external APIs** at the boundary so domain logic doesn't marshal
  foreign types — one place absorbs upstream changes and gives you a seam.
- **Model the domain**: value objects instead of long primitive parameter
  lists, domain types (`Duration`, `Date`, `FoodOrder`) instead of primitive
  obsession — code that matches the product's ideas survives requirement
  churn and asserts cleanly.
- **Make interfaces hard to misuse**: maintain your own invariants (no
  "call `init` first", no "check remaining slots before insert");
  compiler-enforced contracts beat runtime checks beat documentation.
- **Offer the strong guarantee where cheap** (mutate visible state only after
  everything fallible has succeeded — build-then-swap): all-or-nothing
  outcomes are far easier to assert.
- **Don't DRY production code prematurely**: coincidentally-similar code that
  serves different concepts should stay separate until a real shared
  abstraction emerges (YAGNI).
- **Separate restructuring from features** (*prefactoring*): first a
  refactor-only change that makes the feature easy, then the feature — each
  small, reviewable, and safely revertible. Keep pull requests small and
  focused on one thing.
- **TDD (red–green–refactor)** is a reliable way to get these properties:
  the failing test proves the test can fail; writing it first forces a
  testable design.

## Before you ship a test

- Does the name state the scenario *and* the expected outcome?
- Can you name the concrete bug that would make it fail?
- Would it survive a behavior-preserving refactor?
- Are the inputs literal, non-default, and distinct from each other?
- Does it assert outcomes, verifying only state-changing calls?
- Would the failure message alone tell you where to look?
