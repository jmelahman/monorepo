/*
Package typesafe is a client for the TypeSafe API.

TypeSafe answers typed questions about content. You supply a [SystemOneRequest]
holding some state and a set of named questions; the response holds one answer per
question, keyed by the names you chose.

Create a client with [New]. The API key is required and is read from the
TYPESAFE_API_KEY environment variable when [WithAPIKey] is not supplied:

	client, err := typesafe.New()
	if err != nil {
		log.Fatal(err)
	}

There are three kinds of question, built with [Noul], [Choice], and [Score]:

	res, err := client.SystemOne(ctx, typesafe.SystemOneRequest{
		State: "I was charged twice. Please help.",
		Questions: typesafe.Questions{
			"billing": typesafe.Noul("Is this message about billing?"),
			"tone": typesafe.Choice("What is the tone?", typesafe.ChoiceCriteria{
				"angry": "An upset or hostile message",
				"calm":  "A neutral or polite message",
			}),
			"urgency": typesafe.Score("How urgent is this?", typesafe.ScoreCriteria{
				"Can wait", "Needs attention this week", "Needs attention today",
			}),
		},
	})

Read answers with the typed getters, which report a missing name and a wrong
answer kind as distinct errors:

	billing, err := res.Noul("billing")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(billing.Noul)

The client retries 408, 429, and 5xx responses along with connection failures,
honoring Retry-After. See [RetryPolicy] for the defaults and [WithRetry] to change
them. Configuration falls back to the environment variables named by [EnvAPIKey],
[EnvBaseURL], [EnvDefaultModel], and [EnvLogLevel].

A *Client is safe for concurrent use by multiple goroutines.
*/
package typesafe
