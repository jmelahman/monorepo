// Package kevtest stands in for a Kev checkpoint, which is gigabytes and takes
// seconds to answer, in tests of what happens around one.
package kevtest

import (
	"context"
	"strings"
	"sync"

	kev "github.com/jmelahman/kev-go"

	"github.com/jmelahman/kanban/internal/releases"
)

// Loader loads a model that gives Visible as the probability of an answer of
// yes, or fails with OpenErr or DecideErr. It records what happened.
type Loader struct {
	mu        sync.Mutex
	Visible   float64
	OpenErr   error
	DecideErr error
	opens     int
	closes    int
	states    []string
}

// Open is a releases.OpenModel.
func (l *Loader) Open() (releases.Model, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.opens++
	if l.OpenErr != nil {
		return nil, l.OpenErr
	}
	return model{l}, nil
}

// Set changes the probability given from now on.
func (l *Loader) Set(visible float64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Visible = visible
}

// Counts returns how many times the model was opened and closed.
func (l *Loader) Counts() (opens, closes int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.opens, l.closes
}

// Read reports whether any state the model was given contains text.
func (l *Loader) Read(text string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, state := range l.states {
		if strings.Contains(state, text) {
			return true
		}
	}
	return false
}

type model struct{ l *Loader }

func (m model) Decide(_ context.Context, state string, _ []kev.Question) ([][]float64, int, error) {
	m.l.mu.Lock()
	defer m.l.mu.Unlock()
	m.l.states = append(m.l.states, state)
	if m.l.DecideErr != nil {
		return nil, 0, m.l.DecideErr
	}
	return [][]float64{{1 - m.l.Visible, m.l.Visible}}, 1, nil
}

func (m model) Close() {
	m.l.mu.Lock()
	defer m.l.mu.Unlock()
	m.l.closes++
}
