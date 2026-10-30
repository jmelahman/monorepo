package server

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/jmelahman/kanban/internal/client"
)

// releasesRefreshEvery is how often the view re-reads the summary, so
// reminders and runs from elsewhere show up.
var releasesRefreshEvery = 5 * time.Second

const (
	minReleaseViewWidth  = 60
	minReleaseViewHeight = 10
)

// promptReleaseView takes over the terminal with the release view until the
// user closes it. Releases started from the view keep running.
func promptReleaseView(ctx context.Context, url string) error {
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
	return runReleaseView(ctx, screen, url)
}

// Events the background work posts into the view's loop. All view state is
// touched only on the loop.
type (
	releasesLoaded struct {
		summary client.ReleaseSummary
		err     error
	}
	releasePreviewed struct {
		reminder client.ReleaseReminder
		preview  client.ReleasePreview
		err      error
	}
	releaseStarted struct {
		run client.ReleaseRun
		err error
	}
	releaseDismissed struct{ err error }
	releasesTick     struct{}
	releasesWake     struct{}
)

type releaseActionKind int

const (
	releaseActionPreview releaseActionKind = iota
	releaseActionRun
	releaseActionDismiss
)

// releaseAction is a side effect a key asked for.
type releaseAction struct {
	kind     releaseActionKind
	reminder client.ReleaseReminder
	preview  client.ReleasePreview
}

// releaseController owns the side effects behind a releaseView.
type releaseController struct {
	ctx    context.Context
	c      *client.Client
	screen tcell.Screen
	v      *releaseView

	loading      bool
	streamRun    int64
	streamCancel context.CancelFunc
	wakePending  atomic.Bool
}

// runReleaseView is the event loop, split from promptReleaseView so tests can
// drive it with a tcell.SimulationScreen.
func runReleaseView(ctx context.Context, screen tcell.Screen, url string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctl := &releaseController{ctx: ctx, c: client.New(url, nil), screen: screen, v: &releaseView{}}
	defer ctl.stopStream()
	ctl.load()
	go func() {
		t := time.NewTicker(releasesRefreshEvery)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				ctl.post(releasesTick{})
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
			if act := ctl.v.handleKey(ev); act != nil {
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

func (ctl *releaseController) post(data any) {
	_ = ctl.screen.PostEvent(tcell.NewEventInterrupt(data))
}

func (ctl *releaseController) load() {
	if ctl.loading {
		return
	}
	ctl.loading = true
	go func() {
		summary, err := ctl.c.ReleaseSummary(ctl.ctx)
		ctl.post(releasesLoaded{summary: summary, err: err})
	}()
}

func (ctl *releaseController) do(act releaseAction) {
	r := act.reminder
	switch act.kind {
	case releaseActionPreview:
		ctl.v.setBusy("fetching " + r.Prefix + "…")
		go func() {
			p, err := ctl.c.PreviewRelease(ctl.ctx, r.RepoPath, r.Prefix, r.Bump, r.Suffix)
			ctl.post(releasePreviewed{reminder: r, preview: p, err: err})
		}()
	case releaseActionRun:
		ctl.v.setBusy("starting " + r.Prefix + " " + act.preview.Version + "…")
		go func() {
			run, err := ctl.c.RunRelease(ctl.ctx, client.RunReleaseArgs{
				Repo: r.RepoPath, Prefix: r.Prefix, Bump: r.Bump, Suffix: r.Suffix,
				Version: act.preview.Version, Rev: act.preview.Rev, ReminderID: &r.ID,
			})
			ctl.post(releaseStarted{run: run, err: err})
		}()
	case releaseActionDismiss:
		ctl.v.setBusy("dismissing " + r.Prefix + "…")
		go func() {
			ctl.post(releaseDismissed{err: ctl.c.DismissReleaseReminder(ctl.ctx, r.ID)})
		}()
	}
}

func (ctl *releaseController) apply(data any) {
	v := ctl.v
	switch ev := data.(type) {
	case releasesLoaded:
		ctl.loading = false
		if ev.err != nil {
			v.setError(ev.err.Error())
			return
		}
		v.setSummary(ev.summary)
	case releasePreviewed:
		v.busy = ""
		switch {
		case ev.err != nil:
			v.setError(ev.err.Error())
		case ev.preview.Blocked != "":
			v.setError(ev.preview.Blocked)
		default:
			v.confirm = &releaseConfirm{
				text: fmt.Sprintf("Release %s %s at %s? y/n", ev.reminder.Prefix, ev.preview.Version, shortReleaseRev(ev.preview.Rev)),
				act:  releaseAction{kind: releaseActionRun, reminder: ev.reminder, preview: ev.preview},
			}
		}
	case releaseStarted:
		v.busy = ""
		if ev.err != nil {
			v.setError(ev.err.Error())
			return
		}
		v.pinnedRun = ev.run
		v.setMsg("releasing " + ev.run.Prefix + " " + ev.run.Version)
		ctl.load()
	case releaseDismissed:
		v.busy = ""
		if ev.err != nil {
			v.setError(ev.err.Error())
		}
		ctl.load()
	case releasesTick:
		ctl.load()
	case releasesWake:
		ctl.wakePending.Store(false)
	}
}

// syncStream follows the log the view shows.
func (ctl *releaseController) syncStream() {
	run := ctl.v.logRun()
	var runID int64
	if run != nil {
		runID = run.ID
	}
	if runID == ctl.streamRun {
		return
	}
	ctl.stopStream()
	ctl.streamRun = runID
	ctl.v.out = nil
	if runID == 0 {
		return
	}
	out := &taskOutput{runID: runID}
	ctl.v.out = out
	ctx, cancel := context.WithCancel(ctl.ctx)
	ctl.streamCancel = cancel
	go func() {
		err := ctl.c.StreamReleaseRunOutput(ctx, runID, func(line string) {
			out.append(line)
			if ctl.wakePending.CompareAndSwap(false, true) {
				ctl.post(releasesWake{})
			}
		})
		if ctx.Err() != nil {
			return
		}
		out.finish(err)
		// The run just ended; pick up its status now rather than on the next
		// tick.
		ctl.post(releasesTick{})
	}()
}

func (ctl *releaseController) stopStream() {
	if ctl.streamCancel != nil {
		ctl.streamCancel()
		ctl.streamCancel = nil
	}
}

// releaseConfirm is a question the next key answers.
type releaseConfirm struct {
	text string
	act  releaseAction
}

// releaseView is the state and drawing of the view, with no side effects.
type releaseView struct {
	loaded    bool
	summary   client.ReleaseSummary
	sel       int
	confirm   *releaseConfirm
	busy      string
	msg       string
	errMsg    string
	pinnedRun client.ReleaseRun
	out       *taskOutput
	closed    bool
}

func (v *releaseView) setMsg(s string)   { v.msg, v.errMsg = s, "" }
func (v *releaseView) setError(s string) { v.msg, v.errMsg = "", s }
func (v *releaseView) setBusy(s string) {
	v.busy = s
	v.setMsg("")
}

func (v *releaseView) setSummary(s client.ReleaseSummary) {
	v.loaded, v.summary = true, s
	v.sel = clamp(v.sel, 0, max(len(s.Reminders)-1, 0))
}

func (v *releaseView) selected() *client.ReleaseReminder {
	if v.sel < len(v.summary.Reminders) {
		return &v.summary.Reminders[v.sel]
	}
	return nil
}

// logRun is the run whose log shows: the one just started here, else the
// selected reminder's latest, else the latest of all once no reminder is left.
func (v *releaseView) logRun() *client.ReleaseRun {
	if v.pinnedRun.ID != 0 {
		for i := range v.summary.Runs {
			if v.summary.Runs[i].ID == v.pinnedRun.ID {
				return &v.summary.Runs[i]
			}
		}
		return &v.pinnedRun
	}
	r := v.selected()
	for i := range v.summary.Runs {
		run := &v.summary.Runs[i]
		if r == nil || (run.RepoPath == r.RepoPath && run.Prefix == r.Prefix) {
			return run
		}
	}
	return nil
}

func (v *releaseView) move(delta int) {
	if n := len(v.summary.Reminders); n > 0 {
		v.sel = clamp(v.sel+delta, 0, n-1)
		v.pinnedRun = client.ReleaseRun{}
	}
}

func (v *releaseView) handleKey(ev *tcell.EventKey) *releaseAction {
	if ev.Key() == tcell.KeyCtrlC {
		v.closed = true
		return nil
	}
	if v.confirm != nil {
		confirm := v.confirm
		if ev.Key() == tcell.KeyRune && (ev.Rune() == 'y' || ev.Rune() == 'Y') {
			v.confirm = nil
			return &confirm.act
		}
		if ev.Key() == tcell.KeyEscape || (ev.Key() == tcell.KeyRune && (ev.Rune() == 'n' || ev.Rune() == 'N')) {
			v.confirm = nil
		}
		return nil
	}
	switch ev.Key() { //nolint:exhaustive // tcell.Key is an open keycode space; unlisted keys are deliberately ignored
	case tcell.KeyEscape:
		v.closed = true
	case tcell.KeyUp:
		v.move(-1)
	case tcell.KeyDown:
		v.move(1)
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'q':
			v.closed = true
		case 'k':
			v.move(-1)
		case 'j':
			v.move(1)
		case 'r':
			if r := v.selected(); r != nil && v.busy == "" {
				return &releaseAction{kind: releaseActionPreview, reminder: *r}
			}
		case 'd':
			if r := v.selected(); r != nil && v.busy == "" {
				v.confirm = &releaseConfirm{
					text: fmt.Sprintf("Dismiss the reminder for %s? y/n", r.Prefix),
					act:  releaseAction{kind: releaseActionDismiss, reminder: *r},
				}
			}
		}
	}
	return nil
}

func (v *releaseView) render(s tcell.Screen) {
	s.Clear()
	s.HideCursor()
	w, h := s.Size()
	base := tcell.StyleDefault
	if w < minReleaseViewWidth || h < minReleaseViewHeight {
		putText(s, 0, 0, w, base, "terminal too small for the release view")
		return
	}
	width := w - 2*formPad
	putText(s, formPad, 0, width, base.Bold(true), "Releases")
	if !v.loaded {
		putText(s, formPad, 1, width, base.Dim(true), "loading…")
	} else if v.summary.Unavailable != "" {
		putText(s, formPad, 1, width, base.Foreground(tcell.ColorRed), truncateText(v.summary.Unavailable, width))
	}

	listW := width * 2 / 5
	logX := formPad + listW + 2
	bodyH := h - 5
	v.renderReminders(s, formPad, 3, listW, bodyH)
	v.renderLog(s, logX, 3, w-formPad-logX, bodyH)

	footer := "↑↓ select · r release · d dismiss · q close (releases keep running)"
	putText(s, formPad, h-2, width, base.Dim(true), truncateText(footer, width))
	switch {
	case v.confirm != nil:
		putText(s, formPad, h-1, width, base.Foreground(tcell.ColorYellow).Bold(true), truncateText(v.confirm.text, width))
	case v.busy != "":
		putText(s, formPad, h-1, width, base.Dim(true), truncateText(v.busy, width))
	case v.errMsg != "":
		putText(s, formPad, h-1, width, base.Foreground(tcell.ColorRed).Bold(true), truncateText(v.errMsg, width))
	case v.msg != "":
		putText(s, formPad, h-1, width, base.Foreground(tcell.ColorGreen), truncateText(v.msg, width))
	}
}

func (v *releaseView) renderReminders(s tcell.Screen, x, y, width, height int) {
	base := tcell.StyleDefault
	if v.loaded && len(v.summary.Reminders) == 0 {
		putText(s, x, y, width, base.Dim(true), truncateText("no reminders; add one with kanban release remind", width))
		return
	}
	// Each reminder takes two lines: what, then the note.
	first := max(v.sel-(height/2-1), 0)
	for i := first; i < len(v.summary.Reminders) && (i-first)*2+1 < height; i++ {
		r := v.summary.Reminders[i]
		style := base
		marker := "  "
		if i == v.sel {
			style, marker = base.Bold(true), "▸ "
		}
		line := fmt.Sprintf("%s%s  %s  %s unreleased", marker, r.Prefix, reminderBump(r), unreleasedText(findReleaseSubtree(v.summary, r.RepoPath, r.Prefix)))
		row := y + (i-first)*2
		putText(s, x, row, width, style, truncateText(line, width))
		putText(s, x+2, row+1, width-2, base.Dim(true), truncateText(r.Note, width-2))
	}
}

func (v *releaseView) renderLog(s tcell.Screen, x, y, width, height int) {
	base := tcell.StyleDefault
	run := v.logRun()
	if run == nil {
		putText(s, x, y, width, base.Dim(true), truncateText("no runs yet", width))
		return
	}
	header := fmt.Sprintf("#%d %s %s · %s", run.ID, run.Prefix, run.Version, run.Status)
	putText(s, x, y, width, base.Bold(true), truncateText(header, width))
	row := y + 1
	if run.Error != "" {
		putText(s, x, row, width, base.Foreground(tcell.ColorRed), truncateText(run.Error, width))
		row++
	}
	if v.out == nil {
		return
	}
	lines, _, err := v.out.snapshot()
	if err != nil {
		putText(s, x, row, width, base.Foreground(tcell.ColorRed), truncateText(err.Error(), width))
		row++
	}
	room := y + height - row
	if room <= 0 {
		return
	}
	lines = lines[max(len(lines)-room, 0):]
	for i, line := range lines {
		putText(s, x, row+i, width, base, truncateText(line, width))
	}
}
