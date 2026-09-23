package server

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/jmelahman/kanban/internal/client"
)

func ptrStr(s string) *string { return &s }
func ptrInt(i int64) *int64   { return &i }

// fullTicketInfo exercises every section: a body, a running session with a
// Claude id and a container, a PR, and a port.
func fullTicketInfo() ticketInfo {
	return ticketInfo{
		Ticket: client.Ticket{
			ID: 42, BoardID: 1, ColumnID: 2,
			Title:     "Fix the login bug",
			Slug:      "fix-the-login-bug",
			Body:      "Plus signs in emails 500 the login page.\n\nRepro: sign in as a+b@example.com",
			CreatedAt: 1750000000,
		},
		Board:  infoBoard{ID: 1, Name: "Demo", Slug: "demo"},
		Column: "In Progress",
		Session: &client.Session{
			ID: 7, TicketID: 42,
			WorktreePath:    "/home/dev/worktrees/demo/fix-the-login-bug",
			BranchName:      "kanban/demo/fix-the-login-bug",
			ContainerID:     ptrStr("0123456789abcdef0123"),
			ContainerName:   ptrStr("kanban-demo-fix-the-login-bug"),
			Status:          "working",
			StartedAt:       ptrInt(1750000100),
			PRState:         "open",
			PRNumber:        ptrInt(451),
			PRURL:           "https://github.com/acme/demo/pull/451",
			PRTitle:         "Fix login for plus-addressed emails",
			RepoPath:        "/home/dev/code/demo",
			ClaudeSessionID: "1f0e5a2c-0000-4000-8000-abcdefabcdef",
		},
		Ports: []client.Port{
			{ID: 1, SessionID: 7, Label: "web", ContainerPort: 5173, HostPort: 13001, ProxyActive: true},
			{ID: 2, SessionID: 7, Label: "api", ContainerPort: 7474, HostPort: 13002},
		},
	}
}

// fieldByLabel finds a field across every section, so a test can assert on
// one row without depending on the order of the rest.
func fieldByLabel(t *testing.T, info ticketInfo, label string) infoField {
	t.Helper()
	for _, sec := range info.sections() {
		for _, f := range sec.fields {
			if f.label == label {
				return f
			}
		}
	}
	t.Fatalf("no %q field in %+v", label, info.sections())
	return infoField{}
}

func TestTicketInfoSections(t *testing.T) {
	info := fullTicketInfo()
	var titles []string
	for _, sec := range info.sections() {
		titles = append(titles, sec.title)
	}
	want := []string{"Ticket", "Description", "Session", "Container", "Workspace", "Pull request", "Ports"}
	if strings.Join(titles, ",") != strings.Join(want, ",") {
		t.Errorf("sections = %v, want %v", titles, want)
	}

	// The id displays with a "#" but copies as the bare number a script or
	// another kanban command can take.
	if f := fieldByLabel(t, info, "ID"); f.value != "#42" || f.copy != "42" {
		t.Errorf("ID field = %+v", f)
	}
	if f := fieldByLabel(t, info, "Body"); !strings.HasPrefix(f.copy, "Plus signs") || !strings.Contains(f.copy, "a+b@example.com") {
		t.Errorf("Body copies %q, want the whole body", f.copy)
	}
	if f := fieldByLabel(t, info, "Branch"); f.copy != "kanban/demo/fix-the-login-bug" {
		t.Errorf("Branch field = %+v", f)
	}
	// The PR row shows the human summary but copies title + url, the shape
	// you paste into a chat message.
	f := fieldByLabel(t, info, "PR")
	if !strings.Contains(f.value, "#451") || !strings.Contains(f.value, "(open)") {
		t.Errorf("PR value = %q", f.value)
	}
	if f.copy != "Fix login for plus-addressed emails https://github.com/acme/demo/pull/451" {
		t.Errorf("PR copies %q", f.copy)
	}
	// A proxied port copies the URL you'd actually open.
	if f := fieldByLabel(t, info, "web"); f.copy != "http://localhost:13001" {
		t.Errorf("web port = %+v", f)
	}
	if f := fieldByLabel(t, info, "api"); !strings.Contains(f.value, "(inactive)") {
		t.Errorf("inactive port value = %q", f.value)
	}
	if f := fieldByLabel(t, info, "Created"); !strings.HasPrefix(f.value, "2025-") {
		t.Errorf("Created = %q, want a formatted timestamp", f.value)
	}
}

func TestTicketInfoSectionsWithoutSession(t *testing.T) {
	info := fullTicketInfo()
	info.Session, info.Ports = nil, nil
	info.Ticket.Body = ""
	var titles []string
	for _, sec := range info.sections() {
		titles = append(titles, sec.title)
	}
	if strings.Join(titles, ",") != "Ticket,Description,Session" {
		t.Errorf("sections = %v", titles)
	}
	// Placeholders render but carry nothing to copy, so the viewer skips them.
	if f := fieldByLabel(t, info, "Body"); f.value != "none" || f.copy != "" {
		t.Errorf("empty body field = %+v", f)
	}
	if f := fieldByLabel(t, info, "Status"); f.copy != "" || !strings.Contains(f.value, "no session") {
		t.Errorf("sessionless status = %+v", f)
	}
}

func TestTicketInfoViewNavigateAndCopy(t *testing.T) {
	screen := newFormScreen(t, 80, 24)
	stubNativeCopy(t)
	v := newTicketInfoView(fullTicketInfo())

	if got := v.current().label; got != "ID" {
		t.Fatalf("initial highlight = %q, want ID", got)
	}
	v.handleKey(screen, formKey(tcell.KeyEnter))
	if got := string(screen.GetClipboardData()); got != "42" {
		t.Errorf("copied %q, want 42", got)
	}
	if !strings.Contains(v.msg, "copied ID") {
		t.Errorf("msg = %q", v.msg)
	}

	v.handleKey(screen, formKey(tcell.KeyDown))
	if got := v.current().label; got != "Title" {
		t.Fatalf("after Down: %q, want Title", got)
	}
	// "c" and "y" copy too, and a fresh key clears the previous message.
	v.handleKey(screen, tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))
	if got := string(screen.GetClipboardData()); got != "Fix the login bug" {
		t.Errorf("c copied %q", got)
	}
	v.handleKey(screen, tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone))
	v.handleKey(screen, tcell.NewEventKey(tcell.KeyRune, 'y', tcell.ModNone))
	if got := string(screen.GetClipboardData()); got != "fix-the-login-bug" {
		t.Errorf("y copied %q, want the slug", got)
	}

	v.handleKey(screen, formKey(tcell.KeyEnd))
	if got := v.current().label; got != "api" {
		t.Errorf("End highlights %q, want the last port", got)
	}
	v.handleKey(screen, formKey(tcell.KeyDown)) // clamps at the bottom
	if got := v.current().label; got != "api" {
		t.Errorf("Down at the bottom moved to %q", got)
	}
	v.handleKey(screen, formKey(tcell.KeyHome))
	if got := v.current().label; got != "ID" {
		t.Errorf("Home highlights %q", got)
	}
	v.handleKey(screen, formKey(tcell.KeyUp)) // clamps at the top
	if got := v.current().label; got != "ID" {
		t.Errorf("Up at the top moved to %q", got)
	}

	if v.closed {
		t.Fatal("view closed early")
	}
	v.handleKey(screen, tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone))
	if !v.closed {
		t.Error("q did not close the view")
	}
}

// TestTicketInfoViewSkipsPlaceholders covers a view whose rows are mostly
// "none": the highlight must never land somewhere Enter can't act.
func TestTicketInfoViewSkipsPlaceholders(t *testing.T) {
	screen := newFormScreen(t, 80, 24)
	stubNativeCopy(t)
	info := fullTicketInfo()
	info.Ticket.Body = ""
	info.Session.ContainerID = nil
	info.Session.ContainerName = nil
	info.Ports = nil
	v := newTicketInfoView(info)

	for i := 0; i < len(v.fields)+2; i++ {
		if got := v.current(); got.copy == "" {
			t.Fatalf("highlight landed on placeholder %+v after %d moves", got, i)
		}
		v.handleKey(screen, formKey(tcell.KeyDown))
	}
	for i := 0; i < len(v.fields)+2; i++ {
		if got := v.current(); got.copy == "" {
			t.Fatalf("highlight landed on placeholder %+v moving back", got)
		}
		v.handleKey(screen, formKey(tcell.KeyUp))
	}
}

func TestTicketInfoViewCopyError(t *testing.T) {
	screen := newFormScreen(t, 80, 24)
	restore := nativeCopy
	nativeCopy = func(string) error { return errFakeClipboard }
	t.Cleanup(func() { nativeCopy = restore })

	v := newTicketInfoView(fullTicketInfo())
	v.handleKey(screen, formKey(tcell.KeyEnter))
	if v.msg != "" {
		t.Errorf("claimed success: %q", v.msg)
	}
	if !strings.Contains(v.errMsg, "copy failed") || !strings.Contains(v.errMsg, "xclip exploded") {
		t.Errorf("errMsg = %q", v.errMsg)
	}
}

func TestTicketInfoViewRender(t *testing.T) {
	screen := newFormScreen(t, 70, 40)
	stubNativeCopy(t)
	v := newTicketInfoView(fullTicketInfo())
	v.render(screen)
	screen.Show()

	text := screenText(screen)
	for _, want := range []string{
		"#42 Fix the login bug",
		"Ticket",
		"Board",
		"Demo (demo)",
		"Description",
		"Plus signs in emails 500 the login page.",
		"Session",
		"working",
		"Container",
		"kanban-demo-fix-the-login-bug",
		"Workspace",
		"kanban/demo/fix-the-login-bug",
		"Pull request",
		"#451 Fix login for plus-addressed emails (open)",
		"Ports",
		"container :5173 → host :13001",
		"↑↓ move · Enter copy · Esc close",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("screen missing %q:\n%s", want, text)
		}
	}

	v.handleKey(screen, formKey(tcell.KeyEnter))
	v.render(screen)
	screen.Show()
	if got := screenText(screen); !strings.Contains(got, "copied ID to the clipboard") {
		t.Errorf("copy confirmation missing:\n%s", got)
	}
}

// TestTicketInfoViewScrollsToSelection drives a short screen: the
// highlighted field has to be on it after every move, including the long
// description that spans several rows.
func TestTicketInfoViewScrollsToSelection(t *testing.T) {
	screen := newFormScreen(t, 60, 12)
	stubNativeCopy(t)
	v := newTicketInfoView(fullTicketInfo())
	for i := 0; i < len(v.fields); i++ {
		v.render(screen)
		screen.Show()
		label := v.current().label
		if !strings.Contains(screenText(screen), label) {
			t.Fatalf("field %q off screen at step %d:\n%s", label, i, screenText(screen))
		}
		v.handleKey(screen, formKey(tcell.KeyDown))
	}
	for i := 0; i < len(v.fields); i++ {
		v.handleKey(screen, formKey(tcell.KeyUp))
		v.render(screen)
		screen.Show()
		if label := v.current().label; !strings.Contains(screenText(screen), label) {
			t.Fatalf("field %q off screen scrolling back:\n%s", label, screenText(screen))
		}
	}
}

func TestTicketInfoViewTinyTerminal(t *testing.T) {
	screen := newFormScreen(t, 20, 5)
	v := newTicketInfoView(fullTicketInfo())
	v.render(screen)
	screen.Show()
	if got := screenText(screen); !strings.Contains(got, "terminal too small") {
		t.Errorf("tiny terminal render = %q", got)
	}
}

func TestWriteTicketInfo(t *testing.T) {
	var out bytes.Buffer
	if err := writeTicketInfo(&out, fullTicketInfo()); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"#42 Fix the login bug (fix-the-login-bug)",
		"\nTicket\n",
		"\nDescription\n",
		"Repro: sign in as a+b@example.com",
		"\nPull request\n",
		"\nPorts\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("plain output missing %q:\n%s", want, got)
		}
	}
	for _, row := range [][2]string{
		{"Title", "Fix the login bug"},
		{"Board", "Demo (demo)"},
		{"Branch", "kanban/demo/fix-the-login-bug"},
		{"web", "container :5173 → host :13001  http://localhost:13001"},
	} {
		if !hasPlainRow(got, row[0], row[1]) {
			t.Errorf("plain output has no %q row for %q:\n%s", row[0], row[1], got)
		}
	}
	// Continuation lines of the description align under the value column
	// rather than repeating the label.
	if strings.Count(got, "Body ") != 1 {
		t.Errorf("body label repeated:\n%s", got)
	}
}

// hasPlainRow reports whether the plain output has a "<label>   <value>"
// line, without pinning the test to the padding writeTicketInfo computes.
func hasPlainRow(out, label, value string) bool {
	for _, line := range strings.Split(out, "\n") {
		f := strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(f, label+" "); ok && strings.TrimSpace(rest) == value {
			return true
		}
	}
	return false
}

func TestWrapText(t *testing.T) {
	for _, tc := range []struct {
		name  string
		text  string
		width int
		want  []string
	}{
		{"short", "hello", 10, []string{"hello"}},
		{"wraps on spaces", "one two three four", 9, []string{"one two", "three", "four"}},
		{"keeps blank lines", "a\n\nb", 10, []string{"a", "", "b"}},
		{"hard-breaks a long word", "aaaaaaaa", 3, []string{"aaa", "aaa", "aa"}},
		{"empty", "", 10, []string{""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := wrapText(tc.text, tc.width)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("wrapText(%q, %d) = %q, want %q", tc.text, tc.width, got, tc.want)
			}
			for _, line := range got {
				if runesWidth([]rune(line)) > tc.width {
					t.Errorf("line %q exceeds width %d", line, tc.width)
				}
			}
		})
	}
}

var errFakeClipboard = errors.New("xclip exploded")

// stubNativeCopy keeps setClipboard's OSC 52 write (which the simulation
// screen records) while stopping it from reaching the machine's clipboard.
func stubNativeCopy(t *testing.T) {
	t.Helper()
	restore := nativeCopy
	nativeCopy = func(string) error { return nil }
	t.Cleanup(func() { nativeCopy = restore })
}
