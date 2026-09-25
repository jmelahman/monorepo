package server

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/jmelahman/kanban/internal/client"
)

func TestSanitizeOutputLine(t *testing.T) {
	for in, want := range map[string]string{
		"plain":                                "plain",
		"\x1b[32m  VITE\x1b[0m ready":          "  VITE ready",
		"50%\r75%\r100%":                       "100%",
		"trailing cr\r":                        "trailing cr",
		"a\tb":                                 "a b",
		"\x1b]8;;http://x\x07link\x1b]8;;\x07": "link",
		"bell\x07 and \x7fdel":                 "bell and del",
	} {
		if got := sanitizeOutputLine(in); got != want {
			t.Errorf("sanitizeOutputLine(%q) = %q, want %q", in, got, want)
		}
	}
}

func tasksViewRows() []taskRow {
	zero := 0
	return []taskRow{
		{Task: client.Task{Label: "Web", ContainerPort: 5173, HasPort: true}, URL: "http://localhost:13001",
			LastRun: &client.TaskRun{ID: 5, TaskLabel: "Web", Status: "running"}},
		{Task: client.Task{Label: "Tests"},
			LastRun: &client.TaskRun{ID: 4, TaskLabel: "Tests", Status: "exited", ExitCode: &zero}},
		{Task: client.Task{Label: "Docs", ContainerPort: 5175, HasPort: true}},
	}
}

func TestTasksViewActions(t *testing.T) {
	screen := newFormScreen(t, 100, 30)
	v := newTasksView(42)
	v.setRows(tasksViewRows())

	// Enter on a running task stops it.
	if act := v.handleKey(screen, formKey(tcell.KeyEnter)); act == nil || act.kind != actionStop || act.runID != 5 {
		t.Fatalf("Enter on running Web = %+v, want stop run 5", act)
	}
	// Enter on an exited task runs it again.
	v.handleKey(screen, formKey(tcell.KeyDown))
	if act := v.handleKey(screen, formKey(tcell.KeyEnter)); act == nil || act.kind != actionRun || act.label != "Tests" {
		t.Fatalf("Enter on Tests = %+v, want run", act)
	}
	// While that's pending, Enter is refused rather than doubled up.
	v.pending["Tests"] = "starting…"
	if act := v.handleKey(screen, formKey(tcell.KeyEnter)); act != nil || v.errMsg == "" {
		t.Errorf("Enter while pending = %+v (err %q), want refusal", act, v.errMsg)
	}
	// "s" on a task that isn't running explains itself.
	delete(v.pending, "Tests")
	if act := v.handleKey(screen, runeKey('s')); act != nil || !strings.Contains(v.errMsg, "isn't running") {
		t.Errorf("s on exited task = %+v (err %q)", act, v.errMsg)
	}
	// A refresh that reorders the list keeps the highlight on Tests.
	rows := tasksViewRows()
	rows[0], rows[1] = rows[1], rows[0]
	v.setRows(rows)
	if r := v.selected(); r == nil || r.Label != "Tests" {
		t.Errorf("selection after reorder = %+v, want Tests", r)
	}
	v.handleKey(screen, runeKey('q'))
	if !v.closed {
		t.Error("q didn't close the view")
	}
}

func TestTasksViewCopyURL(t *testing.T) {
	var copied string
	restore := setClipboard
	setClipboard = func(_ tcell.Screen, text string) error { copied = text; return nil }
	t.Cleanup(func() { setClipboard = restore })

	screen := newFormScreen(t, 100, 30)
	v := newTasksView(42)
	v.setRows(tasksViewRows())
	v.handleKey(screen, runeKey('c'))
	if copied != "http://localhost:13001" {
		t.Errorf("copied %q", copied)
	}
	v.render(screen)
	screen.Show()
	if got := screenText(screen); !strings.Contains(got, "copied http://localhost:13001 to the clipboard") {
		t.Errorf("footer doesn't confirm the copy:\n%s", got)
	}
	v.handleKey(screen, formKey(tcell.KeyDown))
	v.handleKey(screen, runeKey('c'))
	if !strings.Contains(v.errMsg, "no port") {
		t.Errorf("copy on portless task: err %q", v.errMsg)
	}
	v.handleKey(screen, formKey(tcell.KeyDown))
	v.handleKey(screen, runeKey('y'))
	if !strings.Contains(v.errMsg, "run it first") {
		t.Errorf("copy on unproxied task: err %q", v.errMsg)
	}
	setClipboard = func(tcell.Screen, string) error { return errors.New("boom") }
	v.handleKey(screen, formKey(tcell.KeyHome))
	v.cursor = 0
	v.handleKey(screen, runeKey('c'))
	if !strings.Contains(v.errMsg, "boom") {
		t.Errorf("copy failure: err %q", v.errMsg)
	}
}

func TestTasksViewRender(t *testing.T) {
	screen := newFormScreen(t, 100, 20)
	v := newTasksView(42)
	v.ticket = client.Ticket{ID: 42, Title: "Fix login"}
	v.sessionID, v.sessionStatus, v.loaded = 7, "idle", true
	v.setRows(tasksViewRows())
	v.out = &taskOutput{runID: 5, label: "Web"}
	for i := range 30 {
		v.out.append("line " + string(rune('a'+i%26)))
	}
	v.out.append("\x1b[32mready\x1b[0m")
	v.render(screen)
	screen.Show()
	got := screenLines(screen)
	for _, want := range []string{
		"#42 Fix login · tasks", "session #7 idle",
		"TASK", "Web", "5173", "running (run #5)", "http://localhost:13001",
		"Tests", "exited 0", "Docs",
		"output · Web (run #5)", "ready",
		"Enter run/stop",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("render missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\x1b") {
		t.Error("escape sequence reached the screen")
	}

	// Scrolling up shows older output and says so.
	v.handleKey(screen, formKey(tcell.KeyHome))
	v.render(screen)
	screen.Show()
	got = screenLines(screen)
	if !strings.Contains(got, "line a") || !strings.Contains(got, "scrolled up") || strings.Contains(got, "ready") {
		t.Errorf("after Home:\n%s", got)
	}

	screen.SetSize(20, 5)
	v.render(screen)
	screen.Show()
	if !strings.Contains(screenLines(screen), "too small") {
		t.Error("tiny terminal not handled")
	}
}

// TestTasksViewLoop drives the real loop against the fake API: the list
// loads, Enter runs the highlighted task, its proxied URL and output show
// up, and Esc closes the view without stopping anything.
func TestTasksViewLoop(t *testing.T) {
	restore := tasksRefreshEvery
	tasksRefreshEvery = 20 * time.Millisecond
	t.Cleanup(func() { tasksRefreshEvery = restore })

	f := &fakeTasksAPI{}
	srv := newFakeTasksServer(t, f)
	screen := &shownScreen{SimulationScreen: newFormScreen(t, 110, 24)}

	done := make(chan error, 1)
	go func() { done <- runTicketTasksView(t.Context(), screen, srv.URL, 42) }()

	waitFor := func(want string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for !strings.Contains(screen.text(), want) {
			if time.Now().After(deadline) {
				t.Fatalf("screen never showed %q:\n%s", want, screen.text())
			}
			time.Sleep(5 * time.Millisecond)
		}
	}

	waitFor("session #7 idle")
	waitFor("Tests")
	screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)
	waitFor("hello")
	waitFor("http://127.0.0.1:13001")
	waitFor("exited 0")

	screen.InjectKey(tcell.KeyEscape, 0, tcell.ModNone)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("view didn't close on Esc")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.started) != 1 || f.started[0] != "Web" {
		t.Errorf("started = %v, want [Web]", f.started)
	}
	if len(f.stopped) != 0 {
		t.Errorf("closing the view stopped runs %v", f.stopped)
	}
}

// shownScreen snapshots what the loop last showed, on the loop's own
// goroutine: SimulationScreen isn't safe to read while it's being drawn.
type shownScreen struct {
	tcell.SimulationScreen
	mu    sync.Mutex
	shown string
}

func (s *shownScreen) Show() {
	s.SimulationScreen.Show()
	text := screenLines(s.SimulationScreen)
	s.mu.Lock()
	s.shown = text
	s.mu.Unlock()
}

func (s *shownScreen) text() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shown
}

func runeKey(r rune) *tcell.EventKey { return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone) }
