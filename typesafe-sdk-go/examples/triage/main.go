// Command triage classifies a support message with one TypeSafe request and
// prints the routing decision it would make.
//
// Usage:
//
//	TYPESAFE_API_KEY=... go run ./examples/triage
//	TYPESAFE_API_KEY=... go run ./examples/triage "my message text"
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jmelahman/typesafe-sdk-go"
)

const defaultMessage = `Subject: Charged twice!

Your system billed my card two times this month and I need the second charge
refunded today. I've already emailed once with no reply.`

// urgency is the rubric the score question is graded against. It is shared with
// the reporting below so the levels and the labels cannot drift apart.
var urgency = typesafe.ScoreCriteria{
	"Routine: can wait a week",
	"Normal: should be answered in a day or two",
	"Urgent: needs a reply today",
}

func main() {
	message := defaultMessage
	if len(os.Args) > 1 {
		message = os.Args[1]
	}

	client, err := typesafe.New()
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Every question is independent, so they are asked together in one request
	// and answered in parallel.
	res, err := client.SystemOne(ctx, typesafe.SystemOneRequest{
		State: message,
		Questions: typesafe.Questions{
			"team": typesafe.Choice("Which team should handle this message?", typesafe.ChoiceCriteria{
				"billing": "Payments, invoices, refunds, or incorrect charges",
				"support": "Product problems, bugs, and how-to questions",
				"sales":   "Pricing, plans, and buying decisions",
			}),
			"urgency":  typesafe.Score("How urgently does this need a human reply?", urgency),
			"followup": typesafe.Noul("Is the sender saying they contacted us before without a reply?"),
			"angry": typesafe.Noul("Is the sender angry?").WithCriteria(typesafe.NoulCriteria{
				True:  "Frustration, hostility, threats to leave, or demands",
				False: "Neutral, polite, or merely direct",
			}),
		},
	})
	if err != nil {
		var apiErr *typesafe.APIError
		if errors.As(err, &apiErr) {
			log.Fatalf("the API returned %d: %s (request_id=%s)", apiErr.StatusCode, apiErr.Message, apiErr.RequestID)
		}
		log.Fatal(err)
	}

	team, err := res.Choice("team")
	if err != nil {
		log.Fatal(err)
	}
	level, err := res.Score("urgency")
	if err != nil {
		log.Fatal(err)
	}
	followup, err := res.Noul("followup")
	if err != nil {
		log.Fatal(err)
	}
	angry, err := res.Noul("angry")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("team:     %s (confidence %.2f)\n", team.Choice, team.Confidence)
	fmt.Printf("urgency:  %s (score %.2f)\n", urgency[level.Level()], level.Score)
	fmt.Printf("followup: %.0f%%\n", followup.Noul*100)
	fmt.Printf("angry:    %.0f%%\n", angry.Noul*100)

	// Thresholds are policy and belong in code: the judgments above stay
	// reusable, and changing this rule does not need another request.
	switch {
	case team.Confidence < 0.6:
		fmt.Println("\naction: route to a human triager, the team is unclear")
	case level.Level() == len(urgency)-1 || (angry.Noul > 0.7 && followup.Noul > 0.5):
		fmt.Printf("\naction: page the %s team\n", team.Choice)
	default:
		fmt.Printf("\naction: queue for the %s team\n", team.Choice)
	}

	fmt.Printf("\nmodel %s, %d in / %d out tokens, request %s\n",
		res.Model, res.Usage.InputTokens, res.Usage.OutputTokens, res.Meta.RequestID)
}
