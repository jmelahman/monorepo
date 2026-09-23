# Episode index

75 posts from the [Google Testing Blog](https://testing.googleblog.com/) (2007–2026),
mostly Testing on the Toilet (TotT) episodes, grouped by theme.

Cleaned local copies live at `episodes/<theme>/<title-slug>.md` (not committed —
the posts are Google's copyright). Regenerate them with `scripts/fetch_episodes.py`,
which parses this file for the theme directories and post URLs.

## Change-detectors, fidelity, what to assert (`episodes/what-to-test/`)

- [Change-Detector Tests Considered Harmful](https://testing.googleblog.com/2015/01/testing-on-toilet-change-detector-tests.html) — rewrite tests that mirror production call graphs; assert outcomes.
- [Effective Testing](https://testing.googleblog.com/2014/05/testing-on-toilet-effective-testing.html) — balance fidelity, resilience, precision.
- [SMURF: Beyond the Test Pyramid](https://testing.googleblog.com/2024/10/smurf-beyond-test-pyramid.html) — Speed, Maintainability, Utilization, Reliability, Fidelity.
- [Increase Test Fidelity By Avoiding Mocks](https://testing.googleblog.com/2024/02/increase-test-fidelity-by-avoiding-mocks.html) — real → fake → mock.
- [Prefer Testing Public APIs Over Implementation-Detail Classes](https://testing.googleblog.com/2015/01/testing-on-toilet-prefer-testing-public.html)
- [Test Behaviors, Not Methods](https://testing.googleblog.com/2014/04/testing-on-toilet-test-behaviors-not.html)
- [Test Behavior, Not Implementation](https://testing.googleblog.com/2013/08/testing-on-toilet-test-behavior-not.html)
- [Testing State vs. Testing Interactions](https://testing.googleblog.com/2013/03/testing-on-toilet-testing-state-vs.html)
- [Risk-Driven Testing](https://testing.googleblog.com/2014/05/testing-on-toilet-risk-driven-testing.html)
- [What Makes a Good End-to-End Test?](https://testing.googleblog.com/2016/09/testing-on-toilet-what-makes-good-end.html)
- [Testing UI Logic? Follow the User!](https://testing.googleblog.com/2020/10/testing-on-toilet-testing-ui-logic.html)
- [The Invisible Branch](https://testing.googleblog.com/2008/05/tott-invisible-branch.html) — cover the implicit else.
- [Understanding Your Coverage Data](https://testing.googleblog.com/2008/03/tott-understanding-your-coverage-data.html)
- [Too Many Tests](https://testing.googleblog.com/2008/02/in-movie-amadeus-austrian-emperor.html) — not combinatorics, not two-path line coverage.
- [A Matter of Black and White](https://testing.googleblog.com/2008/08/progressive-developer-knows-that-in.html) — must-pass vs thresholded hard cases.
- [Testing Against Interfaces](https://testing.googleblog.com/2008/07/tott-testing-against-interfaces.html) — shared contract tests per implementation.

## Mocks, fakes, stubs (`episodes/test-doubles/`)

- [Don't Mock Types You Don't Own](https://testing.googleblog.com/2020/07/testing-on-toilet-dont-mock-types-you.html)
- [Exercise Service Call Contracts in Tests](https://testing.googleblog.com/2018/11/testing-on-toilet-exercise-service-call.html)
- [Only Verify State-Changing Method Calls](https://testing.googleblog.com/2017/12/testing-on-toilet-only-verify-state.html)
- [Only Verify Relevant Method Arguments](https://testing.googleblog.com/2018/06/testing-on-toilet-only-verify-relevant.html)
- [Know Your Test Doubles](https://testing.googleblog.com/2013/07/testing-on-toilet-know-your-test-doubles.html)
- [Fake Your Way to Better Tests](https://testing.googleblog.com/2013/06/testing-on-toilet-fake-your-way-to.html)
- [Don't Overuse Mocks](https://testing.googleblog.com/2013/05/testing-on-toilet-dont-overuse-mocks.html)
- [Keep Your Fakes Simple](https://testing.googleblog.com/2009/01/tott-keep-your-fakes-simple.html)
- [Friends You Can Depend On](https://testing.googleblog.com/2008/06/tott-friends-you-can-depend-on.html)
- [Stubs Speed up Your Unit Tests](https://testing.googleblog.com/2007/04/tott-stubs-speed-up-your-unit-tests.html)
- [Contain Your Environment](https://testing.googleblog.com/2008/10/tott-contain-your-environment.html)
- [Defeat Static Cling](https://testing.googleblog.com/2008/06/defeat-static-cling.html)
- [Using Dependency Injection to Avoid Singletons](https://testing.googleblog.com/2008/05/tott-using-dependancy-injection-to.html)
- [Better Stubbing in Python](https://testing.googleblog.com/2007/01/better-stubbing-in-python.html) — parameterize; don't patch globals.
- [Partial Mocks using Forwarding Objects](https://testing.googleblog.com/2009/02/tott-partial-mocks-using-forwarding_19.html)

## Structure, data, assertions, names (`episodes/test-structure/`)

- [Tests Too DRY? Make Them DAMP!](https://testing.googleblog.com/2019/12/testing-on-toilet-tests-too-dry-make.html)
- [Keep Tests Focused](https://testing.googleblog.com/2018/06/testing-on-toilet-keep-tests-focused.html)
- [Keep Cause and Effect Clear](https://testing.googleblog.com/2017/01/testing-on-toilet-keep-cause-and-effect.html)
- [Include Only Relevant Details In Tests](https://testing.googleblog.com/2023/10/include-only-relevant-details-in-tests.html)
- [Cleanly Create Test Data](https://testing.googleblog.com/2018/02/testing-on-toilet-cleanly-create-test.html)
- [Choosing Values for Robust Tests](https://testing.googleblog.com/2026/06/choosing-values-for-robust-tests.html)
- [Don't Put Logic in Tests](https://testing.googleblog.com/2014/07/testing-on-toilet-dont-put-logic-in.html)
- [Data Driven Traps!](https://testing.googleblog.com/2008/09/tott-data-driven-traps.html)
- [What Makes a Good Test?](https://testing.googleblog.com/2014/03/testing-on-toilet-what-makes-good-test.html)
- [Writing Descriptive Test Names](https://testing.googleblog.com/2014/10/testing-on-toilet-writing-descriptive.html)
- [Naming Unit Tests Responsibly](https://testing.googleblog.com/2007/02/tott-naming-unit-tests-responsibly.html)
- [Prefer Narrow Assertions in Unit Tests](https://testing.googleblog.com/2024/04/prefer-narrow-assertions-in-unit-tests.html)
- [Test Failures Should Be Actionable](https://testing.googleblog.com/2024/05/test-failures-should-be-actionable.html)
- [How I Learned To Stop Writing Brittle Tests and Love Expressive APIs](https://testing.googleblog.com/2024/04/how-i-learned-to-stop-writing-brittle.html)
- [Truth: a fluent assertion framework](https://testing.googleblog.com/2014/12/testing-on-toilet-truth-fluent.html)
- [Literate Testing With Matchers](https://testing.googleblog.com/2009/09/tott-literate-testing-with-matchers.html)
- [Making a Perfect Matcher](https://testing.googleblog.com/2009/10/tott-making-perfect-matcher.html)
- [EXPECT vs. ASSERT](https://testing.googleblog.com/2008/07/tott-expect-vs-assert.html)
- [Floating-Point Comparison](https://testing.googleblog.com/2008/10/tott-floating-point-comparison.html)
- [The Stroop Effect](https://testing.googleblog.com/2008/02/tott-stroop-effect.html) — failure messages state expected behavior.
- [Refactoring Tests in the Red](https://testing.googleblog.com/2007/04/tott-refactoring-tests-in-red.html)

## Flakiness, time, isolation (`episodes/flakiness/`)

- [Avoiding Flakey Tests](https://testing.googleblog.com/2008/04/tott-avoiding-flakey-tests.html)
- [Sleeping != Synchronization](https://testing.googleblog.com/2008/08/tott-sleeping-synchronization.html)
- [Time is Random](https://testing.googleblog.com/2008/04/tott-time-is-random.html)
- [Simulating Time in jsUnit Tests](https://testing.googleblog.com/2008/10/tott-simulating-time-in-jsunit-tests.html)
- [ThreadSanitizer: Slaughtering Data Races](https://testing.googleblog.com/2014/06/threadsanitizer-slaughtering-data-races.html) — ordinary tests miss races; run detectors.

## Testability of production (seams) (`episodes/testability/`)

- [Functional Core, Imperative Shell](https://testing.googleblog.com/2025/10/simplify-your-code-functional-core.html)
- [Construct with Collaborators, Call with Work](https://testing.googleblog.com/2026/05/construct-with-collaborators-call-with.html)
- [The Way of TDD](https://testing.googleblog.com/2026/03/the-way-of-tdd.html)
- [Separation of Concerns? That's a Wrap!](https://testing.googleblog.com/2020/12/testing-on-toilet-separation-of.html)
- [Avoid Hardcoding Values for Better Libraries](https://testing.googleblog.com/2020/08/testing-on-toilet-avoid-hardcoding.html)
- [Make Interfaces Hard to Misuse](https://testing.googleblog.com/2018/07/code-health-make-interfaces-hard-to.html)
- [Obsessed With Primitives?](https://testing.googleblog.com/2017/11/obsessed-with-primitives.html)
- [Write Change-Resilient Code With Domain Objects](https://testing.googleblog.com/2024/09/write-change-resilient-code-with-domain.html)
- [Don't DRY Your Code Prematurely](https://testing.googleblog.com/2024/05/dont-dry-your-code-prematurely.html)
- [Avoid the Long Parameter List](https://testing.googleblog.com/2024/05/avoid-long-parameter-list.html)
- [Extracting Methods to Simplify Testing](https://testing.googleblog.com/2007/06/tott-extracting-methods-to-simplify.html)
- [Avoiding friend Twister in C++](https://testing.googleblog.com/2007/10/tott-avoiding-friend-twister-in-c.html)
- [Web Testing Made Easier: Debug IDs](https://testing.googleblog.com/2014/08/testing-on-toilet-web-testing-made.html)
- [Be an MVP of GUI Testing](https://testing.googleblog.com/2009/02/with-all-sport-drug-scandals-of-late.html)
- [Testing GWT without GwtTestCase](https://testing.googleblog.com/2009/08/tott-testing-gwt-without-gwttest.html) — keep UI-framework runners off business logic.
- [Testable Contracts Make Exceptional Neighbors](https://testing.googleblog.com/2008/05/tott-testable-contracts-make.html)
- [Prefactoring](https://testing.googleblog.com/2026/07/prefactoring-clear-way-for-your-new.html) — restructure, then feature; don't mix.
- [In Praise of Small Pull Requests](https://testing.googleblog.com/2024/07/in-praise-of-small-pull-requests.html)
