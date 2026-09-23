// Package flag mirrors the subset of solod.dev/so/flag that check-symlinks
// uses, backed by Go's flag package. Solod's FlagSet is a value (its flags
// live in a fixed-size array), so this one is too, wrapping a pointer.
package flag

import (
	goflag "flag"
)

// ErrHelp is the sentinel Parse returns for -h/-help. It is Go's own so that
// callers comparing with == keep working under either toolchain.
var ErrHelp = goflag.ErrHelp

type ErrorHandling int

const (
	ContinueOnError ErrorHandling = iota
	ExitOnError
	PanicOnError
)

type FlagSet struct {
	fs *goflag.FlagSet
}

func NewFlagSet(name string, errorHandling ErrorHandling) FlagSet {
	return FlagSet{fs: goflag.NewFlagSet(name, goflag.ErrorHandling(errorHandling))}
}

func (f *FlagSet) Init(name string, errorHandling ErrorHandling) {
	f.fs = goflag.NewFlagSet(name, goflag.ErrorHandling(errorHandling))
}

// ensure makes the zero FlagSet usable, matching solod, where the zero value
// is a valid FlagSet with ContinueOnError handling.
func (f *FlagSet) ensure() {
	if f.fs == nil {
		f.fs = goflag.NewFlagSet("", goflag.ContinueOnError)
	}
}

func (f *FlagSet) BoolVar(p *bool, name string, value bool, usage string) {
	f.ensure()
	f.fs.BoolVar(p, name, value, usage)
}

func (f *FlagSet) IntVar(p *int, name string, value int, usage string) {
	f.ensure()
	f.fs.IntVar(p, name, value, usage)
}

func (f *FlagSet) Int64Var(p *int64, name string, value int64, usage string) {
	f.ensure()
	f.fs.Int64Var(p, name, value, usage)
}

func (f *FlagSet) StringVar(p *string, name string, value string, usage string) {
	f.ensure()
	f.fs.StringVar(p, name, value, usage)
}

func (f *FlagSet) Float64Var(p *float64, name string, value float64, usage string) {
	f.ensure()
	f.fs.Float64Var(p, name, value, usage)
}

func (f *FlagSet) Parse(arguments []string) error {
	f.ensure()
	return f.fs.Parse(arguments)
}

func (f *FlagSet) Parsed() bool {
	f.ensure()
	return f.fs.Parsed()
}

func (f *FlagSet) Args() []string {
	f.ensure()
	return f.fs.Args()
}

func (f *FlagSet) Arg(i int) string {
	f.ensure()
	return f.fs.Arg(i)
}

func (f *FlagSet) NArg() int {
	f.ensure()
	return f.fs.NArg()
}

func (f *FlagSet) NFlag() int {
	f.ensure()
	return f.fs.NFlag()
}

func (f *FlagSet) Name() string {
	f.ensure()
	return f.fs.Name()
}

func (f *FlagSet) Usage() {
	f.ensure()
	f.fs.Usage()
}

func (f *FlagSet) PrintDefaults() {
	f.ensure()
	f.fs.PrintDefaults()
}
