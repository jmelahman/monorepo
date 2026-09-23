package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/jmelahman/kanban/internal/client"
)

// ---------- data ----------

// ticketInfo is everything `kanban ticket info` shows: the same material as
// the web UI's Info tab, assembled from the ticket, the board it sits on,
// and the session that works on it (which may not exist yet).
type ticketInfo struct {
	Ticket  client.Ticket   `json:"ticket"`
	Board   infoBoard       `json:"board"`
	Column  string          `json:"column"`
	Session *client.Session `json:"session,omitempty"`
	Ports   []client.Port   `json:"ports,omitempty"`
}

type infoBoard struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// loadTicketInfo gathers a ticket's info from the API. The ticket lookup is
// board-free, so an id from anywhere works; everything else hangs off the
// board that lookup names.
func loadTicketInfo(ctx context.Context, url string, ticketID int64) (ticketInfo, error) {
	var info ticketInfo
	c := client.New(url, nil)

	raw, err := c.GetTicket(ctx, ticketID)
	if err != nil {
		return info, err
	}
	if err := json.Unmarshal(raw, &info.Ticket); err != nil {
		return info, fmt.Errorf("decode ticket: %w", err)
	}

	state, err := c.BoardState(ctx, info.Ticket.BoardID)
	if err != nil {
		return info, err
	}
	var st struct {
		Board   infoBoard `json:"board"`
		Columns []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"columns"`
		Sessions []client.Session `json:"sessions"`
	}
	if err := json.Unmarshal(state, &st); err != nil {
		return info, fmt.Errorf("decode board state: %w", err)
	}
	info.Board = st.Board
	for _, col := range st.Columns {
		if col.ID == info.Ticket.ColumnID {
			info.Column = col.Name
			break
		}
	}
	for i := range st.Sessions {
		if st.Sessions[i].TicketID == info.Ticket.ID {
			info.Session = &st.Sessions[i]
			break
		}
	}
	if info.Session != nil {
		// Ports are supporting detail: a failure here (a session torn down
		// between the two calls, say) shouldn't cost the user the rest of
		// the view, so it renders an empty Ports section instead.
		info.Ports, _ = c.ListPorts(ctx, info.Session.ID)
	}
	return info, nil
}

// ---------- fields ----------

// infoField is one line of the view: a label, the text shown next to it,
// and the text a copy puts on the clipboard. A field with no copy text is
// a placeholder ("none", "—") and is skipped when moving the highlight.
type infoField struct {
	label string
	value string
	copy  string
}

// infoSection groups fields the way the web UI's Info tab does.
type infoSection struct {
	title  string
	fields []infoField
}

// plain builds a field whose displayed and copied text are the same.
func plain(label, value string) infoField {
	return infoField{label: label, value: value, copy: value}
}

// muted builds a placeholder field with nothing worth copying.
func muted(label, value string) infoField {
	return infoField{label: label, value: value}
}

// sections lays a ticketInfo out for display. Both the interactive view and
// the plain-text output render this, so the two never drift.
func (info ticketInfo) sections() []infoSection {
	t := info.Ticket
	ticket := []infoField{
		{label: "ID", value: "#" + strconv.FormatInt(t.ID, 10), copy: strconv.FormatInt(t.ID, 10)},
		plain("Title", t.Title),
		plain("Slug", t.Slug),
		plain("Board", formatBoardLabel(info.Board.Name, info.Board.Slug)),
	}
	if info.Column != "" {
		ticket = append(ticket, plain("Column", info.Column))
	}
	ticket = append(ticket, timeField("Created", &t.CreatedAt))
	if t.ArchivedAt != nil {
		ticket = append(ticket, timeField("Archived", t.ArchivedAt))
	}
	out := []infoSection{{title: "Ticket", fields: ticket}}

	if body := strings.TrimRight(t.Body, "\n"); body != "" {
		out = append(out, infoSection{title: "Description", fields: []infoField{plain("Body", body)}})
	} else {
		out = append(out, infoSection{title: "Description", fields: []infoField{muted("Body", "none")}})
	}

	s := info.Session
	if s == nil {
		out = append(out, infoSection{title: "Session", fields: []infoField{
			muted("Status", "no session — run `kanban ticket attach` to start one"),
		}})
		return out
	}

	session := []infoField{
		plain("Status", s.Status),
		{label: "Session ID", value: strconv.FormatInt(s.ID, 10), copy: strconv.FormatInt(s.ID, 10)},
	}
	if s.ClaudeSessionID != "" {
		session = append(session, plain("Claude session", s.ClaudeSessionID))
	}
	session = append(session, timeField("Started", s.StartedAt))
	if s.StoppedAt != nil {
		session = append(session, timeField("Stopped", s.StoppedAt))
	}
	out = append(out, infoSection{title: "Session", fields: session})

	out = append(out, infoSection{title: "Container", fields: []infoField{
		optional("Name", s.ContainerName),
		optional("ID", s.ContainerID),
	}})

	workspace := []infoField{plain("Branch", s.BranchName), plain("Worktree", s.WorktreePath)}
	switch {
	case s.RepoPath != "":
		workspace = append(workspace, plain("Repo", s.RepoPath))
	case s.MountPath != "":
		workspace = append(workspace, plain("Mount", s.MountPath))
	}
	out = append(out, infoSection{title: "Workspace", fields: workspace})

	if s.PRNumber != nil && s.PRURL != "" {
		pr := "#" + strconv.FormatInt(*s.PRNumber, 10)
		if s.PRTitle != "" {
			pr += " " + s.PRTitle
		}
		if s.PRState != "" {
			pr += " (" + s.PRState + ")"
		}
		out = append(out, infoSection{title: "Pull request", fields: []infoField{
			{label: "PR", value: pr, copy: s.PRTitle + " " + s.PRURL},
			plain("URL", s.PRURL),
		}})
	}

	ports := make([]infoField, 0, len(info.Ports))
	for _, p := range info.Ports {
		value := fmt.Sprintf("container :%d → host :%d", p.ContainerPort, p.HostPort)
		local := fmt.Sprintf("http://localhost:%d", p.HostPort)
		if p.ProxyActive {
			value += "  " + local
		} else {
			value += "  (inactive)"
		}
		ports = append(ports, infoField{label: p.Label, value: value, copy: local})
	}
	if len(ports) == 0 {
		ports = append(ports, muted("Ports", "none allocated"))
	}
	out = append(out, infoSection{title: "Ports", fields: ports})
	return out
}

// optional renders a pointer string field, falling back to a placeholder.
func optional(label string, v *string) infoField {
	if v == nil || *v == "" {
		return muted(label, "none")
	}
	return plain(label, *v)
}

// timeField renders a unix timestamp (seconds or milliseconds, as the API
// mixes both) in the local zone. Copying yields the same rendered string.
func timeField(label string, ts *int64) infoField {
	if ts == nil || *ts == 0 {
		return muted(label, "—")
	}
	ms := *ts
	if ms < 1e12 {
		ms *= 1000
	}
	return plain(label, time.UnixMilli(ms).Format("2006-01-02 15:04:05 MST"))
}

// ---------- command ----------

// runTicketInfo loads a ticket's info and shows it: the interactive viewer
// on a terminal, plain text (or JSON) otherwise, so the command stays usable
// in a pipe.
func runTicketInfo(ctx context.Context, url string, out io.Writer, ticketID int64, asJSON bool) error {
	info, err := loadTicketInfo(ctx, url, ticketID)
	if err != nil {
		return err
	}
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	}
	if !stdinIsTerminal() {
		return writeTicketInfo(out, info)
	}
	return promptTicketInfo(info)
}

// writeTicketInfo prints the same sections the viewer shows as plain text.
func writeTicketInfo(out io.Writer, info ticketInfo) error {
	sections := info.sections()
	width := 0
	for _, sec := range sections {
		for _, f := range sec.fields {
			if n := len(f.label); n > width {
				width = n
			}
		}
	}
	if _, err := fmt.Fprintf(out, "#%d %s (%s)\n", info.Ticket.ID, info.Ticket.Title, info.Ticket.Slug); err != nil {
		return err
	}
	for _, sec := range sections {
		if _, err := fmt.Fprintf(out, "\n%s\n", sec.title); err != nil {
			return err
		}
		for _, f := range sec.fields {
			lines := strings.Split(f.value, "\n")
			for i, line := range lines {
				label := ""
				if i == 0 {
					label = f.label
				}
				if _, err := fmt.Fprintf(out, "  %-*s  %s\n", width, label, line); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// promptTicketInfo takes over the terminal with a tcell screen and runs the
// viewer until the user closes it.
func promptTicketInfo(info ticketInfo) error {
	screen, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("open terminal: %w", err)
	}
	if err := screen.Init(); err != nil {
		return fmt.Errorf("init terminal: %w", err)
	}
	// Fini restores the terminal; make sure it runs even if the loop panics,
	// otherwise the shell is left in raw mode with the alternate screen up.
	defer func() {
		screen.Fini()
		if r := recover(); r != nil {
			panic(r)
		}
	}()
	return runTicketInfoView(screen, newTicketInfoView(info))
}

// runTicketInfoView is the event loop, split from promptTicketInfo so tests
// can drive it with a tcell.SimulationScreen.
func runTicketInfoView(screen tcell.Screen, v *ticketInfoView) error {
	for {
		v.render(screen)
		screen.Show()
		switch ev := screen.PollEvent().(type) {
		case nil:
			// Screen was finalized underneath us.
			return nil
		case *tcell.EventResize:
			screen.Sync()
		case *tcell.EventKey:
			v.handleKey(screen, ev)
		}
		if v.closed {
			return nil
		}
	}
}

// ---------- model ----------

// ticketInfoView is the state behind the viewer: the flattened fields, which
// one is highlighted, and the last copy's outcome. Like ticketForm and
// ticketPicker it keeps no screen state beyond the scroll offset render()
// owns, so key handling is unit-testable without a terminal.
type ticketInfoView struct {
	info     ticketInfo
	sections []infoSection
	fields   []infoField // flattened, in section order
	cursor   int         // index into fields
	msg      string
	errMsg   string
	closed   bool

	// Owned by render(): the first screen row shown and how many body rows
	// fit, which PgUp/PgDn use as their stride.
	scroll   int
	pageRows int
}

func newTicketInfoView(info ticketInfo) *ticketInfoView {
	v := &ticketInfoView{info: info, sections: info.sections(), pageRows: 10}
	for _, sec := range v.sections {
		v.fields = append(v.fields, sec.fields...)
	}
	v.cursor = v.nextCopyable(0, 1)
	return v
}

// nextCopyable returns the first index from start (inclusive) walking in
// direction step that has something to copy, or start clamped when there is
// none — placeholders shouldn't trap or attract the highlight.
func (v *ticketInfoView) nextCopyable(start, step int) int {
	for i := start; i >= 0 && i < len(v.fields); i += step {
		if v.fields[i].copy != "" {
			return i
		}
	}
	// Nothing that way; try the other, so a view of only placeholders still
	// lands somewhere valid.
	for i := start; i >= 0 && i < len(v.fields); i -= step {
		if v.fields[i].copy != "" {
			return i
		}
	}
	return clamp(start, 0, len(v.fields)-1)
}

func (v *ticketInfoView) move(delta int) {
	if len(v.fields) == 0 {
		return
	}
	step := 1
	if delta < 0 {
		step = -1
	}
	target := clamp(v.cursor+delta, 0, len(v.fields)-1)
	v.cursor = v.nextCopyable(target, step)
}

// current returns the highlighted field, or the zero field when there is
// nothing to show.
func (v *ticketInfoView) current() infoField {
	if v.cursor < 0 || v.cursor >= len(v.fields) {
		return infoField{}
	}
	return v.fields[v.cursor]
}

// copyCurrent puts the highlighted field's value on the clipboard and
// records what to say in the footer.
func (v *ticketInfoView) copyCurrent(s tcell.Screen) {
	f := v.current()
	if f.copy == "" {
		v.errMsg = "nothing to copy on this row"
		return
	}
	if err := setClipboard(s, f.copy); err != nil {
		v.errMsg = "copy failed: " + err.Error()
		return
	}
	v.msg = "copied " + f.label + " to the clipboard"
}

// handleKey applies one key event. Bindings:
//
//	Up / Down, Ctrl+P / Ctrl+N, j / k   move the highlight
//	PgUp / PgDn, Home / End             move by a page / to either end
//	Enter, c, y                         copy the highlighted value
//	Esc / q / Ctrl+C                    close
func (v *ticketInfoView) handleKey(s tcell.Screen, ev *tcell.EventKey) {
	v.msg, v.errMsg = "", ""
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		v.closed = true
	case tcell.KeyEnter, tcell.KeyCtrlJ:
		v.copyCurrent(s)
	case tcell.KeyUp, tcell.KeyCtrlP:
		v.move(-1)
	case tcell.KeyDown, tcell.KeyCtrlN:
		v.move(1)
	case tcell.KeyPgUp:
		v.move(-v.pageRows)
	case tcell.KeyPgDn:
		v.move(v.pageRows)
	case tcell.KeyHome:
		v.cursor = v.nextCopyable(0, 1)
	case tcell.KeyEnd:
		v.cursor = v.nextCopyable(len(v.fields)-1, -1)
	case tcell.KeyRune:
		if ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt|tcell.ModMeta) != 0 {
			return
		}
		switch ev.Rune() {
		case 'q':
			v.closed = true
		case 'c', 'y':
			v.copyCurrent(s)
		case 'j':
			v.move(1)
		case 'k':
			v.move(-1)
		case 'g':
			v.cursor = v.nextCopyable(0, 1)
		case 'G':
			v.cursor = v.nextCopyable(len(v.fields)-1, -1)
		}
	}
}

// ---------- rendering ----------

const (
	infoBodyRow    = 2
	infoFooterRows = 3 // blank + help + message
	minInfoWidth   = 30
	minInfoHeight  = 8
	maxInfoLabel   = 16
)

// infoScreenRow is one screen line: a section heading, a blank spacer, or
// one wrapped line of a field. field indexes v.fields, or -1.
type infoScreenRow struct {
	heading string
	field   int
	label   string
	text    string
}

// layout flattens the sections into screen rows at the given value width,
// soft-wrapping multi-line values (the description, mostly) so a long field
// occupies as many rows as it needs and the highlight covers all of them.
func (v *ticketInfoView) layout(labelW, valueW int) []infoScreenRow {
	var rows []infoScreenRow
	field := 0
	for si, sec := range v.sections {
		if si > 0 {
			rows = append(rows, infoScreenRow{field: -1})
		}
		rows = append(rows, infoScreenRow{heading: sec.title, field: -1})
		for range sec.fields {
			f := v.fields[field]
			label := truncateText(f.label, labelW)
			for _, line := range wrapText(f.value, valueW) {
				rows = append(rows, infoScreenRow{field: field, label: label, text: line})
				label = ""
			}
			field++
		}
	}
	return rows
}

func (v *ticketInfoView) render(s tcell.Screen) {
	s.Clear()
	s.HideCursor()
	w, h := s.Size()
	base := tcell.StyleDefault
	if w < minInfoWidth || h < minInfoHeight {
		putText(s, 0, 0, w, base, "terminal too small for the ticket info")
		return
	}
	width := w - 2*formPad

	t := v.info.Ticket
	putText(s, formPad, 0, width, base.Bold(true),
		truncateText(fmt.Sprintf("#%d %s", t.ID, t.Title), width))

	labelW := 0
	for _, f := range v.fields {
		if n := len(f.label); n > labelW {
			labelW = n
		}
	}
	labelW = min(labelW, maxInfoLabel)
	valueW := max(width-labelW-4, 8)

	bodyH := max(h-infoFooterRows-infoBodyRow, 1)
	v.pageRows = bodyH
	rows := v.layout(labelW, valueW)

	// Keep the whole highlighted field on screen when it fits, and at least
	// its first row when it doesn't.
	first, last := len(rows), -1
	for i, r := range rows {
		if r.field == v.cursor {
			if i < first {
				first = i
			}
			last = i
		}
	}
	if last >= 0 {
		if last-first+1 <= bodyH && last >= v.scroll+bodyH {
			v.scroll = last - bodyH + 1
		}
		if first < v.scroll {
			v.scroll = first
		}
		if first >= v.scroll+bodyH {
			v.scroll = first
		}
	}
	v.scroll = clamp(v.scroll, 0, max(0, len(rows)-bodyH))

	for i := v.scroll; i < len(rows) && i-v.scroll < bodyH; i++ {
		y := infoBodyRow + i - v.scroll
		row := rows[i]
		switch {
		case row.heading != "":
			putText(s, formPad, y, width, base.Bold(true).Dim(true), row.heading)
		case row.field < 0:
			// spacer
		default:
			style := base
			if row.field == v.cursor {
				style = style.Reverse(true)
				for x := 0; x < width; x++ {
					s.SetContent(formPad+x, y, ' ', nil, style)
				}
			}
			putText(s, formPad+2, y, labelW, style.Dim(row.field != v.cursor), row.label)
			putText(s, formPad+2+labelW+2, y, valueW, style, row.text)
		}
	}

	putText(s, formPad, h-2, width, base.Dim(true), "↑↓ move · Enter copy · Esc close")
	switch {
	case v.errMsg != "":
		putText(s, formPad, h-1, width, base.Foreground(tcell.ColorRed).Bold(true), v.errMsg)
	case v.msg != "":
		putText(s, formPad, h-1, width, base.Foreground(tcell.ColorGreen), v.msg)
	}
}

// wrapText splits text into display lines of at most width cells, breaking
// on explicit newlines first and then on spaces (falling back to a hard
// break for a word longer than the field).
func wrapText(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	var out []string
	for _, para := range strings.Split(text, "\n") {
		if para == "" {
			out = append(out, "")
			continue
		}
		line := ""
		for _, word := range strings.Fields(para) {
			switch {
			case line == "":
				line = word
			case runesWidth([]rune(line))+1+runesWidth([]rune(word)) <= width:
				line += " " + word
			default:
				out = append(out, line)
				line = word
			}
			for runesWidth([]rune(line)) > width {
				cut := hardCut(line, width)
				out = append(out, line[:cut])
				line = line[cut:]
			}
		}
		out = append(out, line)
	}
	if len(out) == 0 {
		out = []string{""}
	}
	return out
}

// hardCut returns the byte offset that fills at most width cells of s, used
// for words too long to wrap on a space.
func hardCut(s string, width int) int {
	pos, last := 0, 0
	for i, r := range s {
		rw := runeWidth(r)
		if pos+rw > width {
			return max(i, 1)
		}
		pos += rw
		last = i + len(string(r))
	}
	return max(last, 1)
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	return min(max(v, lo), hi)
}
