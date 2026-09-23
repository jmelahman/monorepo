// Package greeter is the worked example that ships with golang-template.
//
// It exists so that every layer of the template — a public API with
// functional options, a sentinel error, a table test, a runnable example,
// and a CLI that consumes the package — has one small thing to copy from.
// Delete it and write your own package; keep the shape.
package greeter

import (
	"errors"
	"strings"
)

// ErrEmptyName is returned by [Greeter.Greet] when the name is blank. Export
// sentinel errors so callers can match them with [errors.Is] instead of
// comparing strings.
var ErrEmptyName = errors.New("greeter: empty name")

// Greeter renders greetings. The zero value greets with an empty salutation;
// build one with [New] instead.
//
// Field order matters: `aligo check ./...` runs in CI and on push, and it
// wants wide fields first and bools last so the struct packs tightly.
type Greeter struct {
	greeting string
	shout    bool
}

// New returns a [Greeter] configured by opts. Without options it greets with
// "Hello".
func New(opts ...Option) *Greeter {
	g := &Greeter{greeting: "Hello"}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// Greet renders a greeting for name. It returns [ErrEmptyName] if name is
// empty or only whitespace.
func (g *Greeter) Greet(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrEmptyName
	}
	out := g.greeting + ", " + name + "!"
	if g.shout {
		out = strings.ToUpper(out)
	}
	return out, nil
}

// Option configures a [Greeter]. Functional options keep the constructor
// source-compatible as the package grows: adding a knob adds a function, not
// a parameter.
type Option func(*Greeter)

// WithGreeting replaces the default "Hello" salutation.
func WithGreeting(greeting string) Option {
	return func(g *Greeter) { g.greeting = greeting }
}

// WithShout renders the greeting in upper case.
func WithShout() Option {
	return func(g *Greeter) { g.shout = true }
}
