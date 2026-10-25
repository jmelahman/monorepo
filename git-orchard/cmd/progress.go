package cmd

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

// progress keeps a line on a terminal, below what's printed above it,
// counting work off: how much is done, for how long, and what's under way.
// It does nothing unless its writer is a terminal.
type progress struct {
	w     io.Writer
	label string
	total int

	mu      sync.Mutex
	done    int
	running []string
	start   time.Time
	frame   int
	stop    chan struct{}
	stopped chan struct{}
}

var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// newProgress starts a progress line on w for total items of work, e.g.
// "Comparing subtrees", or returns a progress that does nothing if w isn't a
// terminal. Stop it when the work is done.
func newProgress(w io.Writer, label string, total int) *progress {
	p := &progress{w: w, label: label, total: total, start: time.Now()}
	if !isTerminal(w) {
		return p
	}
	p.stop = make(chan struct{})
	p.stopped = make(chan struct{})
	go func() {
		defer close(p.stopped)
		t := time.NewTicker(100 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-p.stop:
				return
			case <-t.C:
				p.mu.Lock()
				p.frame++
				p.draw()
				p.mu.Unlock()
			}
		}
	}()
	return p
}

func (p *progress) on() bool { return p.stop != nil }

// Begin notes that work on name has started.
func (p *progress) Begin(name string) {
	if !p.on() {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.running = append(p.running, name)
	p.draw()
}

// End notes that work on name is done.
func (p *progress) End(name string) {
	if !p.on() {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.done++
	if i := slices.Index(p.running, name); i >= 0 {
		p.running = slices.Delete(p.running, i, i+1)
	}
	p.draw()
}

// Println prints a line to out above the progress line. out is usually
// stdout, and the progress line on stderr, sharing the terminal.
func (p *progress) Println(out io.Writer, line string) {
	if !p.on() {
		_, _ = fmt.Fprintln(out, line)
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clear()
	_, _ = fmt.Fprintln(out, line)
	p.draw()
}

// Stop clears the progress line for good.
func (p *progress) Stop() {
	if !p.on() {
		return
	}
	close(p.stop)
	<-p.stopped
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clear()
}

// width is the terminal's width, asked each time since it can be resized,
// or a narrow terminal's if it can't be told.
func (p *progress) width() int {
	if f, ok := p.w.(*os.File); ok {
		if w, _, err := term.GetSize(int(f.Fd())); err == nil && w > 10 {
			return w
		}
	}
	return 80
}

func (p *progress) clear() {
	_, _ = fmt.Fprint(p.w, "\r\x1b[K")
}

// draw rewrites the progress line; p.mu must be held.
func (p *progress) draw() {
	line := fmt.Sprintf("%s %s %d/%d (%ds)", spinner[p.frame%len(spinner)], p.label, p.done, p.total,
		int(time.Since(p.start).Seconds()))
	if len(p.running) > 0 {
		line += ": " + strings.Join(p.running, ", ")
	}
	// Past the terminal's width the line would wrap, and \r wouldn't return
	// to its start. The last column is left free too, since some terminals
	// wrap on reaching it.
	if r, w := []rune(line), p.width()-1; len(r) > w {
		line = string(r[:w-1]) + "…"
	}
	p.clear()
	_, _ = fmt.Fprint(p.w, line)
}
