package typesafe_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/jmelahman/typesafe-sdk-go"
)

// Ask one yes/no question about a piece of text. The API key comes from
// TYPESAFE_API_KEY.
func Example() {
	client, err := typesafe.New()
	if err != nil {
		log.Fatal(err)
	}

	res, err := client.SystemOne(context.Background(), typesafe.SystemOneRequest{
		State:     "I was charged twice for my subscription.",
		Questions: typesafe.Questions{"billing": typesafe.Noul("Is this message about billing?")},
	})
	if err != nil {
		log.Fatal(err)
	}

	billing, err := res.Noul("billing")
	if err != nil {
		log.Fatal(err)
	}
	if billing.Noul > 0.8 {
		fmt.Println("route to billing")
	}
}

// Several independent questions are answered in parallel in one request, so ask
// everything you need about a state at once.
func ExampleClient_SystemOne() {
	client, err := typesafe.New(
		typesafe.WithAPIKey("sk-example"),
		typesafe.WithBaseURL(exampleServer()),
	)
	if err != nil {
		log.Fatal(err)
	}

	res, err := client.SystemOne(context.Background(), typesafe.SystemOneRequest{
		State: map[string]any{
			"subject": "Charged twice!",
			"body":    "Your system billed my card two times this month. Fix it today.",
		},
		Questions: typesafe.Questions{
			"topic": typesafe.Choice("Which team should handle this message?", typesafe.ChoiceCriteria{
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

	topic, err := res.Choice("topic")
	if err != nil {
		log.Fatal(err)
	}
	urgency, err := res.Score("urgency")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("team %s (confidence %.2f)\n", topic.Choice, topic.Confidence)
	fmt.Printf("urgency level %d: %v\n", urgency.Level(), urgency.Description())
	// Output:
	// team billing (confidence 0.94)
	// urgency level 2: Needs a reply today
}

// The typed getters tell a misspelled name apart from a question of another
// kind, so neither silently yields a zero probability.
func ExampleSystemOneResponse_Noul() {
	res := &typesafe.SystemOneResponse{Answers: typesafe.Answers{
		"topic": typesafe.ChoiceAnswer{Choice: "billing"},
	}}

	if _, err := res.Noul("topics"); err != nil {
		var missing *typesafe.NoAnswerError
		fmt.Println(errors.As(err, &missing), err)
	}
	if _, err := res.Noul("topic"); err != nil {
		var wrongKind *typesafe.AnswerKindError
		fmt.Println(errors.As(err, &wrongKind), err)
	}
	// Output:
	// true typesafe: no answer named "topics"; the response has "topic"
	// true typesafe: answer "topic" is a choice answer, not noul
}

// ChoiceOf and ChoiceAs let a choice question use your own label type, so the
// answer comes back as one of your constants rather than a bare string.
func ExampleChoiceAs() {
	type Team string
	const (
		Billing Team = "billing"
		Support Team = "support"
	)

	client, err := typesafe.New()
	if err != nil {
		log.Fatal(err)
	}

	res, err := client.SystemOne(context.Background(), typesafe.SystemOneRequest{
		State: "I was charged twice for my subscription.",
		Questions: typesafe.Questions{
			"team": typesafe.ChoiceOf("Which team should handle this message?", map[Team]typesafe.Content{
				Billing: "Payments, invoices, refunds, or charges",
				Support: "Product problems and how-to questions",
			}),
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	team, err := typesafe.ChoiceAs[Team](res, "team")
	if err != nil {
		log.Fatal(err)
	}
	switch team.Choice {
	case Billing:
		fmt.Println("route to billing")
	case Support:
		fmt.Println("route to support")
	}
}

// SystemOneBatch asks many independent requests at once, with a limit on how
// many are in flight. Results are in the order of the requests.
func ExampleClient_SystemOneBatch() {
	client, err := typesafe.New()
	if err != nil {
		log.Fatal(err)
	}

	messages := []string{"Buy now!", "Lunch at noon?", "You won a prize!"}
	reqs := make([]typesafe.SystemOneRequest, len(messages))
	for i, m := range messages {
		reqs[i] = typesafe.SystemOneRequest{
			State:     m,
			Questions: typesafe.Questions{"spam": typesafe.Noul("Is this message spam?")},
		}
	}

	for i, r := range client.SystemOneBatch(context.Background(), reqs, 4) {
		if r.Err != nil {
			log.Printf("%q: %v", messages[i], r.Err)
			continue
		}
		spam, err := r.Response.Noul("spam")
		if err != nil {
			log.Printf("%q: %v", messages[i], err)
			continue
		}
		fmt.Printf("%q spam=%.2f\n", messages[i], spam.Noul)
	}
}

// Retries are configurable per client or per call. Use the request context for
// a deadline across all attempts.
func ExampleWithRetry() {
	policy := typesafe.DefaultRetryPolicy()
	policy.MaxRetries = 5
	policy.BackoffMax = 2 * time.Second

	client, err := typesafe.New(typesafe.WithAPIKey("sk-example"), typesafe.WithRetry(policy))
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = client.SystemOne(ctx, typesafe.SystemOneRequest{
		State:     "...",
		Questions: typesafe.Questions{"ok": typesafe.Noul("Is this fine?")},
	}, typesafe.WithRequestTimeout(5*time.Second))
	if err != nil {
		var rateLimit *typesafe.RateLimitError
		if errors.As(err, &rateLimit) {
			fmt.Println("rate limited; retry after", rateLimit.RetryAfter)
		}
	}
}

// exampleServer serves a fixed response so the examples above are runnable
// without an API key. It starts at most one server per test binary.
var exampleServer = sync.OnceValue(func() string {
	const body = `{
		"model": "jev-1",
		"answers": {
			"topic": {"type": "choice", "choice": "billing", "confidence": 0.94,
			          "probabilities": {"billing": 0.94, "support": 0.06}},
			"urgency": {"type": "score", "score": 1.8, "confidence": 0.71,
			            "legend": {"0": "Can wait a week", "1": "Should be answered in a day or two",
			                       "2": "Needs a reply today"},
			            "probabilities": {"0": 0.04, "1": 0.12, "2": 0.84}}
		},
		"usage": {"input_tokens": 96, "output_tokens": 8}
	}`
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, body)
	}))
	return s.URL
})
