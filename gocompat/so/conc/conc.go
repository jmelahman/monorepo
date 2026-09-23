// Package conc mirrors the subset of solod.dev/so/conc that check-symlinks
// uses. Solod threads are pthreads; here they are goroutines, which is the
// one place where the Go build is not a faithful translation — it is a
// faithful implementation of the same contract.
package conc

// Thread is a handle to a running thread. Wait may be called once.
type Thread struct {
	done chan any
}

type ThreadOptions struct {
	StackSize int
}

// Go starts entry(arg) on a new thread.
func Go(entry func(any) any, arg any) Thread {
	th := Thread{done: make(chan any, 1)}
	go run(th, entry, arg)
	return th
}

func GoWith(entry func(any) any, arg any, opts ThreadOptions) Thread {
	_ = opts
	return Go(entry, arg)
}

func run(th Thread, entry func(any) any, arg any) {
	th.done <- entry(arg)
}

// Wait blocks until the thread returns and yields its result.
func (th Thread) Wait() any {
	if th.done == nil {
		return nil
	}
	return <-th.done
}

// Detach abandons the handle; the thread keeps running.
func (th Thread) Detach() {}
