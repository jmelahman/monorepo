//go:build integration

// Integration tests run against the live API. They are behind a build tag so
// that `go test ./...` never touches the network:
//
//	TYPESAFE_API_KEY=... go test -tags=integration ./...
package typesafe_test

import (
	"context"
	"errors"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jmelahman/typesafe-sdk-go"
)

func liveClient(t *testing.T) *typesafe.Client {
	t.Helper()
	if os.Getenv(typesafe.EnvAPIKey) == "" {
		t.Skipf("set %s to run integration tests", typesafe.EnvAPIKey)
	}
	client, err := typesafe.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client
}

func TestIntegrationSystemOne(t *testing.T) {
	client := liveClient(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	rubric := typesafe.ScoreCriteria{
		"Can wait a week",
		"Should be answered in a day or two",
		"Needs a reply today",
	}
	res, err := client.SystemOne(ctx, typesafe.SystemOneRequest{
		State: map[string]any{
			"subject": "Charged twice!",
			"body":    "Your system billed my card two times this month. Fix it today.",
		},
		Questions: typesafe.Questions{
			"billing": typesafe.Noul("Is this message about a billing problem?"),
			"topic": typesafe.Choice("Which team should handle this message?", typesafe.ChoiceCriteria{
				"billing": "Payments, invoices, refunds, or charges",
				"support": "Product problems and how-to questions",
			}),
			"urgency": typesafe.Score("How urgently does this need a reply?", rubric),
		},
	})
	if err != nil {
		t.Fatalf("SystemOne: %v", err)
	}

	if res.Model == "" {
		t.Error("Model is empty, want the resolved model name")
	}
	if res.Usage.InputTokens <= 0 || res.Usage.OutputTokens <= 0 {
		t.Errorf("Usage = %+v, want positive token counts", res.Usage)
	}
	if !strings.HasPrefix(res.Meta.RequestID, "req_") {
		t.Errorf("RequestID = %q, want a req_ prefix", res.Meta.RequestID)
	}

	billing, err := res.Noul("billing")
	if err != nil {
		t.Fatalf("Noul: %v", err)
	}
	if billing.Noul < 0 || billing.Noul > 1 {
		t.Errorf("Noul = %v, want a probability", billing.Noul)
	}

	topic, err := res.Choice("topic")
	if err != nil {
		t.Fatalf("Choice: %v", err)
	}
	if sum := sumFloats(topic.Probabilities); math.Abs(sum-1) > 0.01 {
		t.Errorf("choice probabilities sum to %v, want about 1", sum)
	}

	urgency, err := res.Score("urgency")
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if sum := sumIntKeyed(urgency.Probabilities); math.Abs(sum-1) > 0.01 {
		t.Errorf("score probabilities sum to %v, want about 1", sum)
	}
	// The legend round-trips the rubric that was submitted.
	for level, want := range rubric {
		if got := urgency.Legend[level]; got != want {
			t.Errorf("legend[%d] = %v, want %v", level, got, want)
		}
	}
	if level := urgency.Level(); level < 0 || level >= len(rubric) {
		t.Errorf("Level() = %d, outside the rubric", level)
	}
}

func TestIntegrationListModels(t *testing.T) {
	client := liveClient(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	list, err := client.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(list.Models) == 0 {
		t.Fatal("Models is empty, want at least one model")
	}
	for _, m := range list.Models {
		if m.Name == "" {
			t.Errorf("model %+v has no name", m)
		}
	}
}

func TestIntegrationBadKeyIsAnAuthenticationError(t *testing.T) {
	if os.Getenv(typesafe.EnvAPIKey) == "" {
		t.Skipf("set %s to run integration tests", typesafe.EnvAPIKey)
	}
	client, err := typesafe.New(typesafe.WithAPIKey("sk-definitely-not-valid"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	_, err = client.ListModels(ctx)
	var authErr *typesafe.AuthenticationError
	if !errors.As(err, &authErr) {
		t.Fatalf("ListModels = %v, want *AuthenticationError", err)
	}
}

func sumFloats(m map[string]float64) float64 {
	var total float64
	for _, v := range m {
		total += v
	}
	return total
}

func sumIntKeyed(m map[int]float64) float64 {
	var total float64
	for _, v := range m {
		total += v
	}
	return total
}
