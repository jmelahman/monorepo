package server

import (
	"slices"
)

// strategyOptions is the list the merge picker cycles through with ←/→: the
// merge strategies the board allows, plus which one it configures as its
// default (merge.default_strategy; "" when unset or disabled).
type strategyOptions struct {
	list []string
	def  string
}

// strategyUnset is the picker's strategy index before anything is chosen on
// a board with no default.
const strategyUnset = -1

// newStrategyOptions builds the row's options from the board's enabled
// strategies. A nil result means there is nothing to choose between, so the
// picker leaves the strategy row out and the server resolves the strategy
// on its own.
func newStrategyOptions(enabled []string, def string) *strategyOptions {
	if len(enabled) < 2 {
		return nil
	}
	if !slices.Contains(enabled, def) {
		def = ""
	}
	return &strategyOptions{list: enabled, def: def}
}

// initial is the index the row starts on: the board's default, else
// strategyUnset. A merge can't be undone, so with no default the picker
// makes the user choose rather than guessing one for a bare Enter.
func (o *strategyOptions) initial() int {
	return slices.Index(o.list, o.def)
}

// cycle steps delta entries from i, wrapping at either end. From
// strategyUnset, → lands on the first entry and ← on the last.
func (o *strategyOptions) cycle(i, delta int) int {
	n := len(o.list)
	if i == strategyUnset {
		i = 0
		if delta > 0 {
			delta--
		}
	}
	return ((i+delta)%n + n) % n
}

func (o *strategyOptions) label(i int) string {
	if i == strategyUnset {
		return "not set"
	}
	l := o.list[i]
	if l == o.def {
		l += " (default)"
	}
	return l
}

const (
	strategyRowLabel   = "Strategy"
	strategyUnsetNote  = "←→ to choose"
	strategyUnsetError = "choose a merge strategy with ←/→ first"
)
