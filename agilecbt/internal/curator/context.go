package curator

import (
	"fmt"
	"strings"
	"time"

	"github.com/jmelahman/agilecbt/internal/db"
)

// buildContext renders the fresh per-turn context the curator sees before
// the user's message. It's rebuilt every turn so tool changes show up, and it
// goes after the stable system prompt so the prompt prefix stays cacheable.
func (c *Curator) buildContext(checkin db.Checkin) (string, error) {
	a := c.app
	now := a.Now()
	snap, err := a.TodaySnapshot()
	if err != nil {
		return "", err
	}
	mood, err := a.MoodHistory(7)
	if err != nil {
		return "", err
	}
	roadmap := checkin.Topic == db.TopicRoadmap
	goalStatus := db.GoalActive
	if roadmap {
		goalStatus = "" // planning looks at resting and done goals too
	}
	goals, err := a.Store.ListGoals(goalStatus)
	if err != nil {
		return "", err
	}
	values, err := a.Store.ListValues()
	if err != nil {
		return "", err
	}
	notes, err := a.Store.ListNotes()
	if err != nil {
		return "", err
	}

	goalTitle := map[int64]string{}
	for _, g := range goals {
		goalTitle[g.ID] = g.Title
	}
	valueName := map[int64]string{}
	for _, v := range values {
		valueName[v.ID] = v.Name
	}

	var b strings.Builder
	b.WriteString("<context>\n")
	fmt.Fprintf(&b, "Now: %s, %s. ", now.Format("Monday"), now.Format("2006-01-02 15:04"))
	if roadmap {
		b.WriteString("This is a roadmap conversation from the Roadmap page, not a check-in.\n")
		if m, err := a.Store.LatestCheckin(snap.Date, "", ""); err == nil {
			fmt.Fprintf(&b, "Today's check-in: %s\n", readings(m.Mood, m.Energy, m.Anxiety))
		}
	} else {
		fmt.Fprintf(&b, "This is a %s check-in", checkin.Kind)
		if checkin.Date != snap.Date {
			fmt.Fprintf(&b, " dated %s", checkin.Date)
		}
		b.WriteString(".\n")
		fmt.Fprintf(&b, "How they're arriving: %s", readings(checkin.Mood, checkin.Energy, checkin.Anxiety))
		if checkin.Note != "" {
			fmt.Fprintf(&b, ". Their note: %q", checkin.Note)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "\nThis week (starting %s)", snap.Week.StartDate)
	if snap.Week.Intention != "" {
		fmt.Fprintf(&b, ", intention: %q", snap.Week.Intention)
	} else {
		b.WriteString(", no intention set yet")
	}
	b.WriteString(".\n")

	b.WriteString("\nToday lane:\n")
	if len(snap.Today) == 0 {
		b.WriteString("  (empty)\n")
	}
	for _, st := range snap.Today {
		fmt.Fprintf(&b, "  %s", stepLine(st.Step, goalTitle))
		if st.CarriedOver {
			b.WriteString(" (from an earlier day)")
		}
		b.WriteString("\n")
	}
	b.WriteString("This-week lane:\n")
	if len(snap.WeekSteps) == 0 {
		b.WriteString("  (empty)\n")
	}
	for _, st := range snap.WeekSteps {
		fmt.Fprintf(&b, "  %s\n", stepLine(st, goalTitle))
	}
	if len(snap.DoneToday) > 0 {
		b.WriteString("Done today:\n")
		for _, st := range snap.DoneToday {
			fmt.Fprintf(&b, "  %s", stepLine(st, goalTitle))
			if st.Mastery != nil || st.Pleasure != nil {
				fmt.Fprintf(&b, " mastery %s, pleasure %s", intOr(st.Mastery), intOr(st.Pleasure))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\nLast 7 days (mood/energy/anxiety, 0-10):\n")
	for _, d := range mood {
		day, _ := time.Parse(time.DateOnly, d.Date)
		fmt.Fprintf(&b, "  %s %s: %s/%s/%s\n", day.Format("Mon"), d.Date, intOr(d.Mood), intOr(d.Energy), intOr(d.Anxiety))
	}

	if roadmap {
		someday, err := a.Store.ListSteps(db.StepFilter{Lanes: []string{db.LaneSomeday}})
		if err != nil {
			return "", err
		}
		b.WriteString("Someday lane:\n")
		if len(someday) == 0 {
			b.WriteString("  (empty)\n")
		}
		for _, st := range someday {
			fmt.Fprintf(&b, "  %s\n", stepLine(st, goalTitle))
		}
		b.WriteString("\nGoals:\n")
	} else {
		b.WriteString("\nActive goals:\n")
	}
	if len(goals) == 0 {
		b.WriteString("  (none yet; the roadmap is empty, so you could gently help them name a value and a first goal)\n")
	}
	for _, g := range goals {
		fmt.Fprintf(&b, "  [goal %d] %s", g.ID, g.Title)
		if roadmap && g.Status != db.GoalActive {
			fmt.Fprintf(&b, " (%s)", g.Status)
		}
		if g.ValueID != nil {
			fmt.Fprintf(&b, " (value: %s)", valueName[*g.ValueID])
		}
		if g.Why != "" {
			fmt.Fprintf(&b, ". Why: %s", g.Why)
		}
		b.WriteString("\n")
	}
	if len(values) > 0 {
		names := make([]string, len(values))
		for i, v := range values {
			names[i] = fmt.Sprintf("[value %d] %s", v.ID, v.Name)
			if roadmap && v.Description != "" {
				names[i] += fmt.Sprintf(" (%s)", v.Description)
			}
		}
		fmt.Fprintf(&b, "Values: %s\n", strings.Join(names, ", "))
	}

	b.WriteString("\nWhat you remember about them:\n")
	if len(notes) == 0 {
		b.WriteString("  (nothing yet)\n")
	}
	for _, n := range notes {
		fmt.Fprintf(&b, "  [note %d] %s\n", n.ID, n.Text)
	}
	b.WriteString("</context>")
	return b.String(), nil
}

func stepLine(st db.Step, goalTitle map[int64]string) string {
	s := fmt.Sprintf("[step %d] %s (energy %d", st.ID, st.Title, st.EnergyCost)
	if st.GoalID != nil {
		if t, ok := goalTitle[*st.GoalID]; ok {
			s += ", goal: " + t
		}
	}
	return s + ")"
}

func readings(mood, energy, anxiety *int) string {
	if mood == nil && energy == nil && anxiety == nil {
		return "not recorded yet"
	}
	return fmt.Sprintf("mood %s, energy %s, anxiety %s", intOr(mood), intOr(energy), intOr(anxiety))
}

func intOr(p *int) string {
	if p == nil {
		return "–"
	}
	return fmt.Sprint(*p)
}
