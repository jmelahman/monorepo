# Worked examples

Original Bad/Good pairs for the rules most often misapplied (see SKILL.md for
the full rule set). Python-flavored unless the rule is framework-specific —
translate to the local language and framework idiom.

## Change-detector test → assert the outcome

```python
# Bad: mirrors the implementation's call graph. Fails on any refactor,
# passes even if the charge amount is computed wrong.
def test_process_order():
    processor = OrderProcessor(mock_inventory, mock_charger)
    processor.process(order)
    mock_inventory.reserve.assert_called_once_with(order.items)
    mock_charger.charge.assert_called_once_with(order.total)
```

```python
# Good: asserts the observable outcome against a fake.
def test_processing_an_order_charges_its_total():
    charger = FakeCardServer()
    processor = OrderProcessor(InMemoryInventory(order.items), charger)
    processor.process(order)
    assert charger.amount_charged(order.id) == order.total
```

## DRY test → DAMP test

```python
# Bad: correct only if you trace setUp and the helper's loop in your head.
def setUp(self):
    self.addresses = [Address("a@x.com"), Address("b@x.com")]
    self.list = MailingList()

def test_subscribe_multiple(self):
    self._subscribe_all()
    for addr in self.addresses:
        self.assertTrue(self.list.is_subscribed(addr))
```

```python
# Good: redundant, and obviously correct on inspection.
def test_can_subscribe_multiple_addresses(self):
    mailing_list = MailingList()
    mailing_list.subscribe(Address("a@x.com"))
    mailing_list.subscribe(Address("b@x.com"))
    self.assertTrue(mailing_list.is_subscribed(Address("a@x.com")))
    self.assertTrue(mailing_list.is_subscribed(Address("b@x.com")))
```

## Broad equality → narrow assertion

```python
# Bad: whole-object equality. Adding any unrelated field to Profile
# breaks this and every test shaped like it.
def test_rename_updates_display_name():
    profile = service.rename(profile_id, "Ada")
    assert profile == Profile(id=profile_id, display_name="Ada",
                              created=CREATED, locale="en", theme="dark")
```

```python
# Good: asserts only the behavior under test.
def test_rename_updates_display_name():
    profile = service.rename(profile_id, "Ada")
    assert profile.display_name == "Ada"
```

## Parameter-list helper → test data builder

```python
# Bad: every new field grows every call site; None-padding hides meaning.
invoice = make_invoice(None, None, "EUR", PAST_DUE, None)
```

```python
# Good: defaults for required fields; tests set only what they assert on.
# (In builder-less languages, a helper with keyword arguments does the same.)
invoice = an_invoice(currency="EUR", status=PAST_DUE)

def an_invoice(**overrides):
    fields = dict(customer=CUSTOMER, total=Money(100), currency="USD",
                  status=OPEN)  # reasonable required defaults
    fields.update(overrides)
    return Invoice(**fields)
```

## Logic in the test → literal expectations

```python
# Bad: the expectation re-implements the code under test; a bug in the
# joining logic exists in both and the test can't see it.
def test_export_path():
    base = "reports/2026/"
    assert exporter.path_for(JUNE) == base + "/june.csv"   # oops: "2026//june"
```

```python
# Good: state the expected value literally; the double slash is now visible.
def test_export_path():
    assert exporter.path_for(JUNE) == "reports/2026/june.csv"
```

## One test, many behaviors → one behavior per test

```python
# Bad: name mirrors the method; unrelated behaviors break together.
def test_signup():
    account = service.signup("ada@x.com")
    assert account.is_active
    assert mailer.sent[0].subject == "Welcome!"
    assert metrics.count("signups") == 1
```

```python
# Good: each behavior named and isolated; failures localize themselves.
def test_signup_activates_the_account():
    assert service.signup("ada@x.com").is_active

def test_signup_sends_a_welcome_email():
    service.signup("ada@x.com")
    assert mailer.sent[0].subject == "Welcome!"

def test_signup_increments_the_signup_counter():
    service.signup("ada@x.com")
    assert metrics.count("signups") == 1
```

## Sleep → explicit synchronization

```python
# Bad: slow when it passes, flaky under load.
def test_upload_completes_in_background():
    uploader.start(file)
    time.sleep(5)
    assert uploader.status(file) == DONE
```

```python
# Good: the test controls when things happen; fast when green,
# times out (with a generous bound) only when red.
def test_upload_completes_in_background():
    done = threading.Event()
    uploader.start(file, on_complete=done.set)
    assert done.wait(timeout=30)
    assert uploader.status(file) == DONE
```

## Values that pass by accident → distinct, non-default values

```python
# Bad: 0 is int's default and both arguments are equal — a store that
# drops the value, or swaps arguments, still passes.
def test_put_stores_value():
    cache.put(0, 0)
    assert cache.get(0) == 0
```

```python
# Good: non-default and distinct per input; dropping or swapping fails.
def test_put_stores_value_under_its_key():
    cache.put(key=7, value=42)
    assert cache.get(7) == 42
```

## Hidden clock → time as an input

```python
# Bad: the current time is a hidden random input; boundary cases
# (month rollover, leap day) are untestable.
def is_expired(subscription):
    return subscription.end_date < date.today()
```

```python
# Good: inject the time; the wrapper keeps callers working.
def is_expired(subscription, today):
    return subscription.end_date < today

def test_subscription_expires_the_day_after_end_date():
    sub = a_subscription(end_date=date(2026, 2, 28))
    assert not is_expired(sub, today=date(2026, 2, 28))
    assert is_expired(sub, today=date(2026, 3, 1))
```
