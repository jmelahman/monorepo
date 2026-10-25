package cmd

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

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

// draw rewrites the progress line; p.mu must be held. It's colored unless
// NO_COLOR is set: the spinner cyan, the label bold, and the elapsed time and
// what's under way dim.
func (p *progress) draw() {
	spin := spinner[p.frame%len(spinner)]
	count := fmt.Sprintf(" %d/%d ", p.done, p.total)
	elapsed := fmt.Sprintf("(%ds)", int(time.Since(p.start).Seconds()))
	running := ""
	if len(p.running) > 0 {
		running = ": " + strings.Join(p.running, ", ")
	}
	// Past the terminal's width the line would wrap, and \r wouldn't return
	// to its start. The last column is left free too, since some terminals
	// wrap on reaching it.
	w := p.width() - 1
	head := spin + " " + p.label + count + elapsed
	if r, n := []rune(head+running), utf8.RuneCountInString(head); len(r) > w {
		if n >= w {
			// Too narrow even for the counts: plain, cut short.
			p.clear()
			_, _ = fmt.Fprint(p.w, string(r[:w-1])+"…")
			return
		}
		running = string(r[n:w-1]) + "…"
	}
	c := painter(os.Getenv("NO_COLOR") == "")
	p.clear()
	_, _ = fmt.Fprint(p.w, c.paint(cyan, spin)+" "+c.paint(bold, p.label)+count+c.paint(dim, elapsed+running))
}
