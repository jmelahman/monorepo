package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/jmelahman/kanban/internal/client"
)

// tasksRefreshEvery is how often the view re-reads the task list, so runs
// started or ended elsewhere (the web UI, another terminal) show up.
var tasksRefreshEvery = 2 * time.Second

// maxTaskOutputLines bounds the output pane's scrollback.
const maxTaskOutputLines = 5000

// ---------- command ----------

// promptTicketTasks takes over the terminal with the task view and runs it
// until the user closes it. Tasks started from the view keep running.
func promptTicketTasks(ctx context.Context, url string, ticketID int64) error {
	screen, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("open terminal: %w", err)
	}
	if err := screen.Init(); err != nil {
		return fmt.Errorf("init terminal: %w", err)
	}
	defer func() {
		screen.Fini()
		if r := recover(); r != nil {
			panic(r)
		}
	}()
	return runTicketTasksView(ctx, screen, url, ticketID)
}

// Events the background work posts into the view's loop, wrapped in a
// tcell.EventInterrupt. All view state is touched only on the loop.
type (
	tasksLoaded struct {
		ticket   client.Ticket
		session  *client.Session
		rows     []taskRow
		warnings []string
		err      error
	}
	taskStarted struct {
		label string
		st    startedTask
		err   error
	}
	taskStopped struct {
		label string
		runID int64
		err   error
	}
	tasksProgress struct{ msg string }
	tasksTick     struct{}
	tasksWake     struct{}
)

// tasksController owns the side effects behind a tasksView: loading, running
// and stopping tasks, and following the selected task's output.
type tasksController struct {
	ctx      context.Context
	c        *client.Client
	url      string
	ticketID int64
	screen   tcell.Screen
	v        *tasksView

	loading      bool
	streamRun    int64
	streamCancel context.CancelFunc
	wakePending  atomic.Bool
}

// runTicketTasksView is the event loop, split from promptTicketTasks so
// tests can drive it with a tcell.SimulationScreen.
func runTicketTasksView(ctx context.Context, screen tcell.Screen, url string, ticketID int64) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctl := &tasksController{
		ctx: ctx, c: client.New(url, nil), url: url, ticketID: ticketID,
		screen: screen, v: newTasksView(ticketID),
	}
	defer ctl.stopStream()
	ctl.load(true)
	go func() {
		t := time.NewTicker(tasksRefreshEvery)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				ctl.post(tasksTick{})
			}
		}
	}()

	for {
		ctl.syncStream()
		ctl.v.render(screen)
		screen.Show()
		switch ev := screen.PollEvent().(type) {
		case nil:
			return nil
		case *tcell.EventResize:
			screen.Sync()
		case *tcell.EventKey:
			if act := ctl.v.handleKey(screen, ev); act != nil {
				ctl.do(*act)
			}
		case *tcell.EventInterrupt:
			ctl.apply(ev.Data())
		}
		if ctl.v.closed {
			return nil
		}
	}
}

func (ctl *tasksController) post(data any) {
	_ = ctl.screen.PostEvent(tcell.NewEventInterrupt(data))
}

// load fetches the ticket's tasks in the background. The first load creates
// the ticket's session (worktree only, no container) when it has none,
// since tasks are discovered from the session's worktree.
func (ctl *tasksController) load(first bool) {
	if ctl.loading {
		return
	}
	ctl.loading = true
	sessionID := ctl.v.sessionID
	go func() {
		ev := tasksLoaded{}
		defer func() { ctl.post(ev) }()
		if first {
			info, err := loadTicketInfo(ctl.ctx, ctl.url, ctl.ticketID)
			if err != nil {
				ev.err = err
				return
			}
			ev.ticket, ev.session = info.Ticket, info.Session
			if ev.session == nil {
				raw, err := ctl.c.EnsureSession(ctl.ctx, ctl.ticketID)
				if err != nil {
					ev.err = err
					return
				}
				var s client.Session
				if err := json.Unmarshal(raw, &s); err != nil {
					ev.err = err
					return
				}
				ev.session = &s
			}
			sessionID = ev.session.ID
		}
		ev.rows, ev.warnings, ev.err = loadTaskRows(ctl.ctx, ctl.url, sessionID)
	}()
}

// do carries out a key's action in the background.
func (ctl *tasksController) do(act tasksAction) {
	switch act.kind {
	case actionRun:
		ctl.v.pending[act.label] = "starting…"
		go func() {
			st, err := startTicketTask(ctl.ctx, ctl.c, ctl.url, progressWriter(ctl.post), ctl.ticketID, act.label)
			ctl.post(taskStarted{label: act.label, st: st, err: err})
		}()
	case actionStop:
		ctl.v.pending[act.label] = "stopping…"
		go func() {
			err := ctl.c.StopTaskRun(ctl.ctx, act.runID)
			ctl.post(taskStopped{label: act.label, runID: act.runID, err: err})
		}()
	}
}

// apply folds a background result into the view.
func (ctl *tasksController) apply(data any) {
	v := ctl.v
	switch ev := data.(type) {
	case tasksLoaded:
		ctl.loading = false
		if ev.err != nil {
			if ctl.ctx.Err() == nil {
				v.setError(ev.err.Error())
			}
			return
		}
		if ev.session != nil {
			v.ticket, v.sessionID, v.sessionStatus = ev.ticket, ev.session.ID, ev.session.Status
		}
		v.loaded = true
		v.setRows(ev.rows)
		if len(ev.warnings) > 0 {
			v.setError("warning: " + strings.Join(ev.warnings, "; "))
		}
	case taskStarted:
		delete(v.pending, ev.label)
		if ev.err != nil {
			v.setError(fmt.Sprintf("run %s: %v", ev.label, ev.err))
			return
		}
		v.sessionID, v.sessionStatus = ev.st.Session.ID, ev.st.Session.Status
		run := ev.st.Run
		v.updateRow(ev.label, func(r *taskRow) {
			r.LastRun = &run
			if ev.st.URL != "" {
				r.URL = ev.st.URL
			}
		})
		switch {
		case ev.st.PortErr != nil:
			v.setError(fmt.Sprintf("started %s, but couldn't proxy port %d: %v", ev.label, ev.st.Task.ContainerPort, ev.st.PortErr))
		case ev.st.URL != "":
			v.setMsg(fmt.Sprintf("started %s → %s", ev.label, ev.st.URL))
		default:
			v.setMsg("started " + ev.label)
		}
		ctl.load(false)
	case taskStopped:
		delete(v.pending, ev.label)
		if ev.err != nil {
			v.setError(fmt.Sprintf("stop %s: %v", ev.label, ev.err))
			return
		}
		v.setMsg(fmt.Sprintf("stopping %s (run #%d)", ev.label, ev.runID))
		ctl.load(false)
	case tasksProgress:
		v.setMsg(ev.msg)
	case tasksTick:
		if v.sessionID != 0 {
			ctl.load(false)
		}
	case tasksWake:
		ctl.wakePending.Store(false)
	}
}

// syncStream follows the selected task's latest run, switching streams when
// the selection or its latest run changes.
func (ctl *tasksController) syncStream() {
	row := ctl.v.selected()
	var runID int64
	if row != nil && row.LastRun != nil {
		runID = row.LastRun.ID
	}
	if runID == ctl.streamRun {
		return
	}
	ctl.stopStream()
	ctl.streamRun = runID
	if runID == 0 {
		ctl.v.out = nil
		return
	}
	out := &taskOutput{runID: runID, label: row.Label}
	ctl.v.out, ctl.v.outScroll = out, 0
	ctx, cancel := context.WithCancel(ctl.ctx)
	ctl.streamCancel = cancel
	go func() {
		err := ctl.c.StreamTaskRunOutput(ctx, runID, func(line string) {
			out.append(line)
			ctl.wake()
		})
		if ctx.Err() != nil {
			return
		}
		out.finish(err)
		// The run just ended; pick up its exit status now rather than on
		// the next tick.
		ctl.post(tasksTick{})
	}()
}

func (ctl *tasksController) stopStream() {
	if ctl.streamCancel != nil {
		ctl.streamCancel()
		ctl.streamCancel = nil
	}
}

// wake asks the loop to redraw, coalescing a burst of output lines (the
// backlog replay, a chatty build) into one event.
func (ctl *tasksController) wake() {
	if ctl.wakePending.CompareAndSwap(false, true) {
		ctl.post(tasksWake{})
	}
}

// progressWriter turns ensureRunningSession's progress lines into footer
// messages.
type progressWriter func(any)

func (p progressWriter) Write(b []byte) (int, error) {
	if msg := strings.TrimSpace(string(b)); msg != "" {
		p(tasksProgress{msg: msg})
	}
	return len(b), nil
}

var _ io.Writer = progressWriter(nil)

// ---------- output buffer ----------

// taskOutput is one run's output as the view shows it. The stream goroutine
// appends; the loop reads when rendering.
type taskOutput struct {
	runID int64
	label string

	mu    sync.Mutex
	lines []string
	done  bool
	err   error
}

func (o *taskOutput) append(line string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.lines = append(o.lines, sanitizeOutputLine(line))
	if n := len(o.lines) - maxTaskOutputLines; n > 0 {
		o.lines = append(o.lines[:0:0], o.lines[n:]...)
	}
}

func (o *taskOutput) finish(err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.done, o.err = true, err
}

func (o *taskOutput) snapshot() (lines []string, done bool, err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.lines, o.done, o.err
}

var ansiEscape = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[@-Z\\-_]`)

// sanitizeOutputLine makes a raw output line safe to draw into cells:
// colour and cursor escapes are dropped, a carriage-return progress line
// keeps only what it last redrew, and other control characters go.
func sanitizeOutputLine(s string) string {
	s = strings.TrimRight(s, "\r")
	if i := strings.LastIndexByte(s, '\r'); i >= 0 {
		s = s[i+1:]
	}
	s = ansiEscape.ReplaceAllString(s, "")
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t':
			return ' '
		case r < 0x20, r == 0x7f:
			return -1
		}
		return r
	}, s)
}

// ---------- model ----------

const (
	actionRun  = "run"
	actionStop = "stop"
)

// tasksAction is what a key asks the controller to do.
type tasksAction struct {
	kind  string
	label string
	runID int64
}

// tasksView is the state behind the task view. Like the other views it
// keeps no screen state beyond what render() owns, so key handling is
// unit-testable without a terminal.
type tasksView struct {
	ticketID      int64
	ticket        client.Ticket
	sessionID     int64
	sessionStatus string
	loaded        bool

	rows    []taskRow
	cursor  int
	pending map[string]string // label → "starting…" / "stopping…"

	out       *taskOutput
	outScroll int // lines scrolled up from the bottom

	msg, errMsg string
	closed      bool

	// Owned by render().
	listScroll int
	outRows    int
}

func newTasksView(ticketID int64) *tasksView {
	return &tasksView{ticketID: ticketID, pending: map[string]string{}, outRows: 10}
}

func (v *tasksView) setMsg(s string)   { v.msg, v.errMsg = s, "" }
func (v *tasksView) setError(s string) { v.msg, v.errMsg = "", s }

// setRows replaces the task list, keeping the highlight on the same task.
func (v *tasksView) setRows(rows []taskRow) {
	label := ""
	if r := v.selected(); r != nil {
		label = r.Label
	}
	v.rows = rows
	v.cursor = clamp(v.cursor, 0, len(rows)-1)
	for i, r := range rows {
		if r.Label == label {
			v.cursor = i
			break
		}
	}
}

func (v *tasksView) updateRow(label string, fn func(*taskRow)) {
	for i := range v.rows {
		if v.rows[i].Label == label {
			fn(&v.rows[i])
		}
	}
}

func (v *tasksView) selected() *taskRow {
	if v.cursor < 0 || v.cursor >= len(v.rows) {
		return nil
	}
	return &v.rows[v.cursor]
}

func (v *tasksView) move(delta int) {
	if len(v.rows) == 0 {
		return
	}
	v.cursor = clamp(v.cursor+delta, 0, len(v.rows)-1)
}

// toggle runs the highlighted task, or stops it when it's running.
func (v *tasksView) toggle() *tasksAction {
	r := v.selected()
	if r == nil {
		return nil
	}
	if p, ok := v.pending[r.Label]; ok {
		v.setError(r.Label + " is " + strings.TrimSuffix(p, "…") + "; wait for it")
		return nil
	}
	if r.LastRun != nil && r.LastRun.Status == "running" {
		return &tasksAction{kind: actionStop, label: r.Label, runID: r.LastRun.ID}
	}
	return &tasksAction{kind: actionRun, label: r.Label}
}

func (v *tasksView) stop() *tasksAction {
	r := v.selected()
	if r == nil {
		return nil
	}
	if r.LastRun == nil || r.LastRun.Status != "running" {
		v.setError(r.Label + " isn't running")
		return nil
	}
	if _, ok := v.pending[r.Label]; ok {
		return nil
	}
	return &tasksAction{kind: actionStop, label: r.Label, runID: r.LastRun.ID}
}

func (v *tasksView) copyURL(s tcell.Screen) {
	r := v.selected()
	switch {
	case r == nil:
		return
	case r.URL == "" && r.HasPort:
		v.setError(r.Label + "'s port isn't proxied yet; run it first")
		return
	case r.URL == "":
		v.setError(r.Label + " has no port in .kanban.toml")
		return
	}
	if err := setClipboard(s, r.URL); err != nil {
		v.setError("copy failed: " + err.Error())
		return
	}
	v.setMsg("copied " + r.URL + " to the clipboard")
}

func (v *tasksView) scrollOutput(delta int) {
	n := 0
	if v.out != nil {
		lines, _, _ := v.out.snapshot()
		n = len(lines)
	}
	v.outScroll = clamp(v.outScroll+delta, 0, max(0, n-v.outRows))
}

// handleKey applies one key event and returns the action it asks for, if
// any. Bindings:
//
//	Up / Down, Ctrl+P / Ctrl+N, j / k   select a task
//	Enter / r                           run the task, or stop it if running
//	s                                   stop the task
//	c / y                               copy the task's URL
//	PgUp / PgDn, Home / End             scroll the output
//	Esc / q / Ctrl+C                    close (tasks keep running)
func (v *tasksView) handleKey(s tcell.Screen, ev *tcell.EventKey) *tasksAction {
	v.msg, v.errMsg = "", ""
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		v.closed = true
	case tcell.KeyEnter, tcell.KeyCtrlJ:
		return v.toggle()
	case tcell.KeyUp, tcell.KeyCtrlP:
		v.move(-1)
	case tcell.KeyDown, tcell.KeyCtrlN:
		v.move(1)
	case tcell.KeyPgUp:
		v.scrollOutput(v.outRows)
	case tcell.KeyPgDn:
		v.scrollOutput(-v.outRows)
	case tcell.KeyHome:
		v.scrollOutput(maxTaskOutputLines)
	case tcell.KeyEnd:
		v.outScroll = 0
	case tcell.KeyRune:
		if ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt|tcell.ModMeta) != 0 {
			return nil
		}
		switch ev.Rune() {
		case 'q':
			v.closed = true
		case 'r':
			return v.toggle()
		case 's':
			return v.stop()
		case 'c', 'y':
			v.copyURL(s)
		case 'j':
			v.move(1)
		case 'k':
			v.move(-1)
		}
	}
	return nil
}

// statusText is a row's status column: a pending action wins over the
// last run's recorded state.
func (v *tasksView) statusText(r taskRow) string {
	if p, ok := v.pending[r.Label]; ok {
		return p
	}
	return runStatus(r.LastRun)
}

// ---------- rendering ----------

const (
	tasksListRow    = 3 // title, session line, blank; then the header
	tasksFooterRows = 2 // help + message
	minTasksWidth   = 40
	minTasksHeight  = 12
	maxTaskLabel    = 32
)

func (v *tasksView) render(s tcell.Screen) {
	s.Clear()
	s.HideCursor()
	w, h := s.Size()
	base := tcell.StyleDefault
	if w < minTasksWidth || h < minTasksHeight {
		putText(s, 0, 0, w, base, "terminal too small for the task view")
		return
	}
	width := w - 2*formPad

	title := fmt.Sprintf("#%d tasks", v.ticketID)
	if v.ticket.Title != "" {
		title = fmt.Sprintf("#%d %s · tasks", v.ticketID, v.ticket.Title)
	}
	putText(s, formPad, 0, width, base.Bold(true), truncateText(title, width))
	switch {
	case !v.loaded:
		putText(s, formPad, 1, width, base.Dim(true), "loading…")
	case v.sessionID != 0:
		line := fmt.Sprintf("session #%d %s", v.sessionID, v.sessionStatus)
		if v.sessionStatus == "stopped" || v.sessionStatus == "error" {
			line += " · running a task starts it"
		}
		putText(s, formPad, 1, width, base.Dim(true), truncateText(line, width))
	}

	// Task list: up to half the body, the rest for output.
	bodyH := h - tasksListRow - tasksFooterRows
	listH := clamp(len(v.rows), 1, max(1, bodyH/2-2))
	v.renderList(s, formPad, tasksListRow, width, listH)

	// Output pane.
	sepY := tasksListRow + 1 + listH + 1
	outH := max(h-tasksFooterRows-sepY-1, 1)
	v.outRows = outH
	v.renderOutput(s, formPad, sepY, width, outH)

	putText(s, formPad, h-2, width, base.Dim(true),
		truncateText("↑↓ select · Enter run/stop · c copy URL · PgUp/PgDn scroll · Esc close (tasks keep running)", width))
	switch {
	case v.errMsg != "":
		putText(s, formPad, h-1, width, base.Foreground(tcell.ColorRed).Bold(true), truncateText(v.errMsg, width))
	case v.msg != "":
		putText(s, formPad, h-1, width, base.Foreground(tcell.ColorGreen), truncateText(v.msg, width))
	}
}

func (v *tasksView) renderList(s tcell.Screen, x, y, width, height int) {
	base := tcell.StyleDefault
	if v.loaded && len(v.rows) == 0 {
		putText(s, x, y, width, base.Dim(true), "no tasks in .vscode/tasks.json or .vscode/launch.json")
		return
	}
	labelW := len("TASK")
	for _, r := range v.rows {
		labelW = max(labelW, runesWidth([]rune(r.Label)))
	}
	labelW = min(labelW, maxTaskLabel)
	statusW := len("STATUS")
	for _, r := range v.rows {
		statusW = max(statusW, len(v.statusText(r)))
	}
	const portW = 5
	cols := func(style tcell.Style, yy int, label, port, status, url string) {
		cx := x + 2
		putText(s, cx, yy, labelW, style, truncateText(label, labelW))
		cx += labelW + 2
		putText(s, cx, yy, portW, style, port)
		cx += portW + 2
		putText(s, cx, yy, statusW, style, status)
		cx += statusW + 2
		putText(s, cx, yy, max(x+width-cx, 0), style, truncateText(url, max(x+width-cx, 0)))
	}
	cols(base.Dim(true), y, "TASK", "PORT", "STATUS", "URL")

	if v.cursor < v.listScroll {
		v.listScroll = v.cursor
	}
	if v.cursor >= v.listScroll+height {
		v.listScroll = v.cursor - height + 1
	}
	v.listScroll = clamp(v.listScroll, 0, max(0, len(v.rows)-height))
	for i := v.listScroll; i < len(v.rows) && i-v.listScroll < height; i++ {
		r := v.rows[i]
		yy := y + 1 + i - v.listScroll
		style := base
		if i == v.cursor {
			style = style.Reverse(true)
			for cx := 0; cx < width; cx++ {
				s.SetContent(x+cx, yy, ' ', nil, style)
			}
			putText(s, x, yy, 1, style, "▸")
		}
		port, url := "-", "-"
		if r.HasPort {
			port = strconv.Itoa(r.ContainerPort)
		}
		if r.URL != "" {
			url = r.URL
		}
		status := v.statusText(r)
		st := style
		switch {
		case strings.HasPrefix(status, "running"):
			st = st.Foreground(tcell.ColorGreen)
		case r.LastRun != nil && r.LastRun.ExitCode != nil && *r.LastRun.ExitCode != 0:
			st = st.Foreground(tcell.ColorRed)
		}
		cols(style, yy, r.Label, port, "", url)
		putText(s, x+2+labelW+2+portW+2, yy, statusW, st, status)
	}
}

func (v *tasksView) renderOutput(s tcell.Screen, x, y, width, height int) {
	base := tcell.StyleDefault
	head := "output"
	var lines []string
	var done bool
	var err error
	if v.out != nil {
		head = fmt.Sprintf("output · %s (run #%d)", v.out.label, v.out.runID)
		lines, done, err = v.out.snapshot()
	}
	if v.outScroll > 0 {
		head += fmt.Sprintf(" · scrolled up %d", v.outScroll)
	}
	head = "── " + head + " "
	putText(s, x, y, width, base.Dim(true), head+strings.Repeat("─", max(width-runesWidth([]rune(head)), 0)))

	body := y + 1
	switch {
	case v.out == nil && v.selected() != nil:
		putText(s, x, body, width, base.Dim(true), "not run yet · Enter runs it")
		return
	case v.out == nil:
		return
	}
	end := len(lines) - v.outScroll
	start := max(end-height, 0)
	for i := start; i < end; i++ {
		putText(s, x, body+i-start, width, base, lines[i])
	}
	if done && v.outScroll == 0 && end-start < height {
		note := "── end of output"
		if err != nil {
			note = "── output stream lost: " + err.Error()
		}
		putText(s, x, body+end-start, width, base.Dim(true), truncateText(note, width))
	}
}
