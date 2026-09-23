# typesafe-sdk-go

A Go client for the [TypeSafe](https://typesafe.ai) API, with no dependencies
outside the standard library.

TypeSafe's System One models answer typed questions about content. You send some
state and a set of named questions; you get back one typed answer per question,
with probabilities, rather than text to parse.

```sh
go get github.com/jmelahman/typesafe-sdk-go
```

Requires Go 1.27 or later.

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jmelahman/typesafe-sdk-go"
)

func main() {
	client, err := typesafe.New() // reads TYPESAFE_API_KEY
	if err != nil {
		log.Fatal(err)
	}

	res, err := client.SystemOne(context.Background(), typesafe.SystemOneRequest{
		State: "Your system billed my card twice this month. Fix it today.",
		Questions: typesafe.Questions{
			"billing": typesafe.Noul("Is this message about a billing problem?"),
			"team": typesafe.Choice("Which team should handle this?", typesafe.ChoiceCriteria{
				"billing": "Payments, invoices, refunds, or charges",
				"support": "Product problems and how-to questions",
			}),
			"urgency": typesafe.Score("How urgently does this need a reply?", typesafe.ScoreCriteria{
				"Can wait a week",
				"Should be answered in a day or two",
				"Needs a reply today",
			}),
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	billing, err := res.Noul("billing")
	if err != nil {
		log.Fatal(err)
	}
	team, err := res.Choice("team")
	if err != nil {
		log.Fatal(err)
	}
	urgency, err := res.Score("urgency")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("billing %.2f, team %s, urgency %d\n", billing.Noul, team.Choice, urgency.Level())
}
```

A runnable version is in [`examples/triage`](examples/triage):

```sh
TYPESAFE_API_KEY=... go run ./examples/triage
```

## Questions

All the questions in one request are answered independently and in parallel, so
ask everything you need about a state at once. Question names are yours; they are
not shown to the model, so each question must carry its full meaning.

| Builder | Answer | Use it for |
|---|---|---|
| `typesafe.Noul(instructions)` | `NoulAnswer{Noul float64}` | whether a condition holds |
| `typesafe.Choice(instructions, criteria)` | `ChoiceAnswer{Choice, Confidence, Probabilities}` | one of a defined set |
| `typesafe.Score(instructions, criteria)` | `ScoreAnswer{Score, Confidence, Legend, Probabilities}` | degree along an ordered rubric |

A Noul answer is the probability that the answer is yes; there is no separate
confidence, and 0.5 means "yes and no are about equally likely", not "moderately
intense". Choice and Score confidence summarize how concentrated the probability
distribution is — not whether the answer is correct for your purpose.

Instructions and criteria descriptions are `Content`: a string, or a JSON object
or array when definitions, contrasts, or examples make the judgment clearer.

```go
typesafe.Noul("Is the sender angry?").WithCriteria(typesafe.NoulCriteria{
	True:  "Frustration, hostility, or threats to leave",
	False: "Neutral, polite, or merely direct",
})

typesafe.Choice("What is the tone?", typesafe.ChoiceCriteria{
	"excited": nil, // nil sends null: interpret the label by its name alone
	"formal":  map[string]any{"means": "measured, businesslike", "not": "cold"},
})
```

Requests are validated locally before they are sent: at least one question,
non-empty choice criteria, and at least two score levels.

### Typed choice labels

Build a choice question with `ChoiceOf` to use your own label type, and read it
with `ChoiceAs` to get the selection back as that type:

```go
type Team string

const (
	Billing Team = "billing"
	Support Team = "support"
)

questions := typesafe.Questions{
	"team": typesafe.ChoiceOf("Which team should handle this?", map[Team]typesafe.Content{
		Billing: "Payments, invoices, refunds, or charges",
		Support: "Product problems and how-to questions",
	}),
}

// after SystemOne:
team, err := typesafe.ChoiceAs[Team](res, "team") // team.Choice is a Team
```

This is safe because response verification (below) has already checked that
the selected label is one of the criteria keys you sent.

## Reading answers

Use the typed getters rather than a type switch. They distinguish a misspelled
name (`*NoAnswerError`, which lists the names that do exist) from a question of
another kind (`*AnswerKindError`), and never hand back a zero value with a nil
error — in a probability API, a silent `0` reads as a confident "no".

```go
answer, err := res.Noul("billing")

var missing *typesafe.NoAnswerError
if errors.As(err, &missing) { /* ... */ }
```

Every response also carries `res.Meta` with the status, headers, request ID, raw
body, attempt count, and the `*http.Response` with its body rewound.

### Response verification

Before `SystemOne` returns, the answers are checked against the questions that
produced them: every question answered, no extra answers, matching kinds, choice
labels and score levels that correspond to the criteria you sent. A mismatch is a
`*ResponseMismatchError` listing each problem.

An answer of a type this SDK does not know becomes an `UnknownAnswer` holding the
original JSON. That is deliberately *not* a mismatch, so a future answer type
does not break existing code.

## Batches

`SystemOneBatch` sends many independent requests with a limit on how many are in
flight, and returns their results in the order of the requests. Each request
succeeds or fails on its own; once the context ends, requests that have not
started fail with its error without being sent.

```go
for i, r := range client.SystemOneBatch(ctx, reqs, 4) {
	if r.Err != nil {
		log.Printf("request %d: %v", i, r.Err)
		continue
	}
	// use r.Response
}
```

Every request retries independently, so a high concurrency multiplies the load
on a rate-limited key.

## Configuration

```go
client, err := typesafe.New(
	typesafe.WithAPIKey(key),
	typesafe.WithBaseURL("https://api.typesafe.ai"),
	typesafe.WithDefaultModel("jev-latest"),
	typesafe.WithTimeout(10*time.Second),
	typesafe.WithRetry(policy),
	typesafe.WithDefaultHeaders(header),
	typesafe.WithHTTPClient(httpClient),
	typesafe.WithLogger(logger),
	typesafe.WithLogLevel(typesafe.LogInfo),
)
```

Each option falls back to an environment variable and then to a default:

| Option | Environment variable | Default |
|---|---|---|
| `WithAPIKey` | `TYPESAFE_API_KEY` | required |
| `WithBaseURL` | `TYPESAFE_BASE_URL` | `https://api.typesafe.ai` |
| `WithDefaultModel` | `TYPESAFE_DEFAULT_MODEL` | `jev-latest` |
| `WithLogLevel` | `TYPESAFE_LOG_LEVEL` | `warn` |
| `WithTimeout` | — | 10s per attempt |

A `*Client` is safe for concurrent use and needs no cleanup. Keep the API key on
the server; it is a bearer credential.

## Retries and deadlines

By default the client makes up to 2 retries, with 500ms exponential backoff
capped at 5s and a quarter of jitter subtracted, for 408, 429, and 5xx responses
(including the API's non-standard 529) as well as connection failures and attempt
timeouts. `Retry-After` and `retry-after-ms` are honored up to 60s.

```go
policy := typesafe.DefaultRetryPolicy()
policy.MaxRetries = 5
client.SystemOne(ctx, req, typesafe.WithRequestRetry(policy))
```

`WithTimeout` bounds a single attempt. The total deadline across all attempts is
the request context — there is no second hidden budget:

```go
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()
```

The client never sleeps into a deadline it will miss, so you see the last real
API error instead of a context error.

## Errors

```go
var apiErr *typesafe.APIError
if errors.As(err, &apiErr) {
	log.Printf("%d %s (request_id=%s)", apiErr.StatusCode, apiErr.Message, apiErr.RequestID)
}
```

`*APIError` matches any HTTP error and carries the status, headers, raw body and
request ID. Match a specific one with `*BadRequestError`, `*AuthenticationError`,
`*PermissionDeniedError`, `*NotFoundError`, `*UnprocessableEntityError`,
`*RateLimitError` (which has `RetryAfter`), or `*InternalServerError`. Transport
problems are `*ConnectionError` and `*TimeoutError`; a caller-cancelled request
unwraps to `context.Canceled` or `context.DeadlineExceeded`. Local mistakes and
invalid bodies are `*Error`, `*ResponseError`, and `*ResponseMismatchError`.

Quote `RequestID` when reporting a problem to TypeSafe.

## Logging

Logging goes to `log/slog`, at `warn` by default. `info` logs one line per
attempt with the method, path, status, duration, request ID and attempt number;
`debug` adds headers and bodies.

Credential headers (`Authorization`, `X-Api-Key`, cookies, anything containing
`token` or `secret`) are redacted. **Bodies are not redacted**, so do not enable
`debug` where your state is sensitive.

## Development

```sh
go build ./... && go vet ./... && go test -race ./...
TYPESAFE_API_KEY=... go test -tags=integration ./...   # hits the live API
```

## License

MIT. See [LICENSE](LICENSE).
