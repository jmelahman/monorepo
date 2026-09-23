// Package atomic mirrors the subset of solod.dev/so/sync/atomic that
// check-symlinks uses, backed by Go's atomic types.
package atomic

import (
	goatomic "sync/atomic"
)

type Bool struct {
	v goatomic.Bool
}

func (x *Bool) Load() bool                        { return x.v.Load() }
func (x *Bool) Store(val bool)                    { x.v.Store(val) }
func (x *Bool) Swap(new bool) bool                { return x.v.Swap(new) }
func (x *Bool) CompareAndSwap(old, new bool) bool { return x.v.CompareAndSwap(old, new) }

type Int64 struct {
	v goatomic.Int64
}

func (x *Int64) Load() int64                        { return x.v.Load() }
func (x *Int64) Store(val int64)                    { x.v.Store(val) }
func (x *Int64) Add(delta int64) int64              { return x.v.Add(delta) }
func (x *Int64) Swap(new int64) int64               { return x.v.Swap(new) }
func (x *Int64) CompareAndSwap(old, new int64) bool { return x.v.CompareAndSwap(old, new) }

type Int32 struct {
	v goatomic.Int32
}

func (x *Int32) Load() int32                        { return x.v.Load() }
func (x *Int32) Store(val int32)                    { x.v.Store(val) }
func (x *Int32) Add(delta int32) int32              { return x.v.Add(delta) }
func (x *Int32) Swap(new int32) int32               { return x.v.Swap(new) }
func (x *Int32) CompareAndSwap(old, new int32) bool { return x.v.CompareAndSwap(old, new) }
