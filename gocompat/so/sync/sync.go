// Package sync mirrors the subset of solod.dev/so/sync that check-symlinks
// uses. Solod's primitives wrap pthread objects, so they have Init and Free;
// Go's are zero-value-ready, so those are no-ops here (Cond.Init still has
// to bind the mutex).
package sync

import (
	gosync "sync"
)

type Mutex struct {
	mu gosync.Mutex
}

func (m *Mutex) Init()         {}
func (m *Mutex) Lock()         { m.mu.Lock() }
func (m *Mutex) TryLock() bool { return m.mu.TryLock() }
func (m *Mutex) Unlock()       { m.mu.Unlock() }
func (m *Mutex) Free()         {}

type Cond struct {
	c *gosync.Cond
}

func (c *Cond) Init(mu *Mutex) { c.c = gosync.NewCond(&mu.mu) }
func (c *Cond) Wait()          { c.c.Wait() }
func (c *Cond) Signal()        { c.c.Signal() }
func (c *Cond) Broadcast()     { c.c.Broadcast() }
func (c *Cond) Free()          {}

type Once struct {
	once gosync.Once
}

func (o *Once) Init()       {}
func (o *Once) Do(f func()) { o.once.Do(f) }
func (o *Once) Free()       {}
