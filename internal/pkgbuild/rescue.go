package pkgbuild

import (
	"bytes"
	"errors"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	"mvdan.cc/sh/v3/syntax"
)

// This file works around known gaps in mvdan.cc/sh's bash parser: constructs
// that bash itself accepts (verified against `bash -n` and the reference
// manual's tokenization rules) but that the upstream lexer rejects. Real
// PKGBUILDs hit these often enough to matter — `provides=($_pkgname=$pkgver)`
// alone accounts for most of them — and a PKGBUILD that fails to parse is a
// PKGBUILD no rule can protect anyone from.
//
// The approach is a same-length byte rewrite: replace each offending byte with
// one the lexer treats as a plain word character, parse the rewritten copy,
// then write the original bytes back into the AST's literal values. Because
// every rewrite is byte-for-byte, every node position in the rescued AST is a
// valid offset into the original source, which is what findings, suppression
// lookups, and --fix's byte-offset edits all rely on. Unit.Raw always keeps
// the original bytes. If the original byte cannot be found in any literal
// after parsing — meaning the rewrite changed structure rather than spelling —
// the rescue declares failure and the caller reports the original parse
// error, so the AST never quietly diverges from the source.

// rescueParse retries a failed parse after minimal byte rewrites. It returns
// nil when no rewrite applies or the rewritten source still does not parse;
// the caller then reports the original error.
func rescueParse(path string, raw []byte) *syntax.File {
	work := bytes.Clone(raw)
	// restore maps a rewritten offset to its original byte, which must
	// resurface in a literal after parsing. structural marks offsets rewritten
	// into statement separators or whitespace, which by design do not come
	// back.
	restore := map[int]byte{}
	structural := map[int]bool{}

	// Upstream's lexer treats `=` and `#` directly after an expansion or a
	// closing quote inside array parens as an assignment operator and a
	// comment respectively. Bash treats both as plain word characters there:
	// an `=` only carries meaning at the start of a word (`a=v`, `[i]=v`) and
	// a `#` only opens a comment at the start of a word. v3.14.0 fixed the
	// same confusion for `[` (`$DLAGENTS[@]`, `tools=[]`); `=` and `#` still
	// lack the word-continuation guard. Rewriting these bytes is therefore
	// semantics-preserving everywhere: wherever the parser already handled
	// them, they sat inside a literal and are restored to an identical AST.
	for _, i := range wordContinuations(raw) {
		restore[i] = work[i]
		work[i] = '~'
	}

	// Upstream's lexer requires the whole file to be valid UTF-8. Bash has no
	// such rule: a byte above ASCII is an ordinary word character whatever its
	// encoding, so a Latin-1 name in a `# Contributor:` line — the shape seen
	// in the wild — is a file bash reads and upstream refuses to tokenize.
	// Each undecodable byte becomes `_`, which is a word character in exactly
	// the same positions, and is restored afterwards.
	for _, i := range invalidUTF8(raw) {
		restore[i] = work[i]
		work[i] = '_'
	}

	var f *syntax.File
	for range 40 {
		var err error
		f, err = newParser().Parse(bytes.NewReader(work), path)
		if err == nil {
			break
		}
		f = nil
		off, msg, ok := errorAt(err)
		if !ok {
			return nil
		}
		if !rescueSubscript(work, off, restore) && !rescueInlineArray(work, msg, off, structural) &&
			!rescueArithFallback(work, msg, off, restore) && !rescueEmptyExpansion(work, msg, off, restore) &&
			!rescueCmdSubstParen(work, off, structural) && !rescueHeredocText(work, off, restore) {
			return nil
		}
	}
	if f == nil || !restoreBytes(f, restore) {
		return nil
	}
	return f
}

// errorAt extracts the byte offset and message of a parse failure.
func errorAt(err error) (int, string, bool) {
	var pe syntax.ParseError
	if errors.As(err, &pe) {
		return int(pe.Pos.Offset()), pe.Text, true
	}
	var le syntax.LangError
	if errors.As(err, &le) {
		return int(le.Pos.Offset()), le.Error(), true
	}
	return 0, "", false
}

func isIdentByte(b byte) bool {
	return b == '_' || ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z') || ('0' <= b && b <= '9')
}

// invalidUTF8 returns the offset of every byte that is not part of a valid
// UTF-8 sequence. Bytes that do decode are left alone, so a file that is UTF-8
// throughout yields nothing and never reaches a rewrite.
func invalidUTF8(raw []byte) []int {
	var out []int
	for i := 0; i < len(raw); {
		if raw[i] < utf8.RuneSelf {
			i++
			continue
		}
		r, size := utf8.DecodeRune(raw[i:])
		if r == utf8.RuneError && size == 1 {
			out = append(out, i)
		}
		i += size
	}
	return out
}

// wordContinuations returns the offsets of every `=` and `#` that continues a
// word begun by an expansion or a quoted part: the previous byte closes one
// (`}`, `"`, `'`) or belongs to a `$name` reference. At such a position bash
// never sees an operator or a comment, only more of the same word. `${x#pat}`
// and `$#` are not matched: their `#` follows `{x` and `$`, neither of which
// ends a `$name`.
func wordContinuations(raw []byte) []int {
	var out []int
	for i := 1; i < len(raw); i++ {
		if b := raw[i]; b != '=' && b != '#' {
			continue
		}
		switch prev := raw[i-1]; {
		case prev == '}' || prev == '"' || prev == '\'':
		case isIdentByte(prev):
			j := i - 1
			for j > 0 && isIdentByte(raw[j-1]) {
				j--
			}
			if j == 0 || raw[j-1] != '$' {
				continue
			}
		default:
			continue
		}
		out = append(out, i)
	}
	return out
}

// rescueSubscript handles a parse failure inside `name[...]`. Upstream parses
// every subscript as arithmetic, but bash defers that until it knows the
// variable's type: after `declare -A sums`, `sums[7.1]=x` indexes with the
// plain string "7.1", which the arithmetic parser rejects as a float. The
// subscript's bytes are blanked to `_`, which parses as a variable name, and
// restored afterwards. The rescued index is an arithmetic variable reference
// rather than a string key — a shape bash itself cannot decide statically —
// but its literal carries the original text, so rendered values stay truthful.
//
// The same key can appear without a name in front of it, as an element of an
// array literal: `declare -A sums=(\n  [7.1]=x\n)`. There the `[` opens a
// word rather than continuing one, so the bracket pair is only accepted when
// it is immediately followed by `=` — the assignment shape a glob is never in.
// Upstream would have taken a glob as a plain literal and not errored here at
// all, so reaching this point already means it read the brackets as a
// subscript.
func rescueSubscript(work []byte, off int, restore map[int]byte) bool {
	if off >= len(work) {
		return false
	}
	lb := -1
	for i := off; i >= 0 && work[i] != '\n'; i-- {
		if work[i] == '[' {
			lb = i
			break
		}
		if work[i] == ']' {
			return false
		}
	}
	if lb <= 0 {
		return false
	}
	element := false
	switch b := work[lb-1]; {
	case isIdentByte(b):
	case b == ' ' || b == '\t' || b == '\n' || b == '(':
		element = true
	default:
		return false
	}
	rb := -1
	for i := lb + 1; i < len(work) && work[i] != '\n'; i++ {
		if work[i] == '[' {
			return false
		}
		if work[i] == ']' {
			rb = i
			break
		}
	}
	// The bracket pair must be a subscript, not a glob: `x[k]=`, `x[k]+=`,
	// `${x[k]}`, or `${x[k]` followed by a parameter-expansion operator
	// (`${_supported[armv8.1-a]:-}`) all continue with a byte a glob never
	// precedes at an offset the parser errored on.
	if rb < 0 || rb == lb+1 || rb+1 >= len(work) {
		return false
	}
	if element {
		if work[rb+1] != '=' {
			return false
		}
	} else {
		switch work[rb+1] {
		case '=', '+', '}', ':', '-', '?', '%', '#', '/', '^', ',':
		default:
			return false
		}
	}
	// An expansion inside the key must not be blanked. Bash expands a
	// subscript when it evaluates the assignment, so `m[1.2$(curl url | sh)]=y`
	// runs that command; flattening it to literal text would leave the rules
	// reading an inert string and reporting nothing. No real key needs one —
	// a subscript that is only a variable reference parses as arithmetic and
	// never reaches this rescue — so the rescue fails closed and the caller
	// reports the original parse error, which grades the package unscanned
	// rather than clean.
	if bytes.ContainsAny(work[lb+1:rb], "$`") {
		return false
	}
	changed := false
	for i := lb + 1; i < rb; i++ {
		if work[i] == '_' {
			continue
		}
		if _, seen := restore[i]; !seen {
			restore[i] = work[i]
		}
		work[i] = '_'
		changed = true
	}
	return changed
}

// rescueInlineArray handles `arr+=( x ) cmd`, which bash parses (the
// assignment lands in the command's temporary environment) but upstream
// rejects outright. The whitespace byte after the array's closing paren
// becomes `;`, splitting one statement into two. This is the one rescue that
// changes statement structure — the temporary-environment scoping is lost —
// which is harmless to rules but means the inserted byte never resurfaces in
// a literal; it is recorded as structural so restoreBytes does not veto it.
func rescueInlineArray(work []byte, msg string, off int, structural map[int]bool) bool {
	if !strings.Contains(msg, "inline variables cannot be arrays") {
		return false
	}
	i := off
	for i < len(work) && (isIdentByte(work[i]) || work[i] == '+') {
		i++
	}
	if i+1 >= len(work) || work[i] != '=' || work[i+1] != '(' {
		return false
	}
	end := arrayEnd(work, i+2)
	if end < 0 || end+1 >= len(work) {
		return false
	}
	j := end + 1
	for j < len(work) && (work[j] == ' ' || work[j] == '\t') {
		j++
	}
	// A whitespace byte to overwrite and a trailing command must both exist,
	// or this is not the shape the error describes.
	if j == end+1 || j >= len(work) || work[j] == '\n' || structural[end+1] {
		return false
	}
	structural[end+1] = true
	work[end+1] = ';'
	return true
}

// rescueArithFallback handles `$((( cmd )) ...)`: a command substitution whose
// first command is an arithmetic command. Bash resolves the `$((` ambiguity by
// attempting an arithmetic expansion and re-reading the construct as a command
// substitution when that parse fails; upstream commits to arithmetic and
// reports the unmatched `))`. The arithmetic command's own paren pairs are
// blanked to `_`, turning it into an ordinary command word, and restored
// afterwards — the rescued statement runs a command spelled `((`, which no
// rule treats specially, and every rendered value keeps the original text.
func rescueArithFallback(work []byte, msg string, off int, restore map[int]byte) bool {
	if !strings.Contains(msg, "without matching `$((` with `))`") {
		return false
	}
	// Only the unambiguous fallback shape: `$(((` where the inner `((...))`
	// pair closes together. Anything else keeps the original error.
	if off+3 >= len(work) || work[off] != '$' || work[off+1] != '(' || work[off+2] != '(' || work[off+3] != '(' {
		return false
	}
	q := arrayEnd(work, off+4)
	if q < 0 || q+1 >= len(work) || work[q+1] != ')' {
		return false
	}
	for _, i := range []int{off + 2, off + 3, q, q + 1} {
		if work[i] == '_' {
			return false // already rewritten once; do not loop
		}
		if _, seen := restore[i]; !seen {
			restore[i] = work[i]
		}
		work[i] = '_'
	}
	return true
}

// rescueEmptyExpansion handles `${}`. Bash decides a parameter expansion's
// name when it expands it, so an empty one is a run-time "bad substitution"
// and not a parse error: `bash -n` accepts a file containing it, and makepkg
// reads such a PKGBUILD's metadata without ever running the line. Upstream
// rejects it while lexing. The `$` is blanked to `_`, leaving the plain word
// `_{}`, and restored afterwards; the rescued node is literal text rather
// than an expansion, which is what an expansion of nothing amounts to.
func rescueEmptyExpansion(work []byte, msg string, off int, restore map[int]byte) bool {
	if !strings.Contains(msg, "invalid parameter name") {
		return false
	}
	// Only the empty spelling. Any other name upstream rejects is one bash
	// rejects too, and must keep the original error.
	if off < 2 || off >= len(work) || work[off] != '}' || work[off-1] != '{' || work[off-2] != '$' {
		return false
	}
	i := off - 2
	if _, seen := restore[i]; !seen {
		restore[i] = work[i]
	}
	work[i] = '_'
	return true
}

// rescueCmdSubstParen handles a command substitution whose first command is a
// subshell, written without the separating space: `$((cd "$srcdir" && pwd) |
// tr -d /)`. Bash resolves the `$((` ambiguity by attempting an arithmetic
// expansion and re-reading the construct as a command substitution when that
// attempt fails; upstream commits to arithmetic and reports either the first
// word of the command as a bad operator or, when the command happens to read
// as arithmetic (`$((ls) | wc -l)`), the missing `))` at the construct's `$`.
// The subshell's own paren pair is blanked to spaces, so the construct lexes
// as an ordinary command substitution whose first command is the subshell's;
// the grouping is lost, which is recorded as structural rather than restored.
//
// rescueArithFallback covers the neighbouring `$((( cmd )) ...)` spelling,
// where the inner command is arithmetic rather than a subshell and upstream
// fails at the end of the construct instead of inside it.
func rescueCmdSubstParen(work []byte, off int, structural map[int]bool) bool {
	// The nearest `$((` opening at or before the failure. Both of upstream's
	// errors land at or after the construct's `$`, so this is the construct
	// it is inside — provided the failure also lies before the construct
	// closes, which is checked below.
	p := -1
	for i := min(off, len(work)-3); i >= 0; i-- {
		if work[i] == '$' && work[i+1] == '(' && work[i+2] == '(' {
			p = i
			break
		}
	}
	if p < 0 {
		return false
	}
	// A `$((` that closed before the failing offset is not the construct
	// upstream is inside, and neither is one sitting in a comment or a
	// quoted string ahead of an unrelated failure. Rewriting it would hand
	// the rules text that is not in the file, since the rewrite below is
	// never restored.
	end := arrayEnd(work, p+2)
	if end < 0 || off > end {
		return false
	}
	// Bash's own discriminator, applied statically: a real arithmetic
	// expansion closes with `))`, so an inner paren that closes alone is a
	// subshell and the construct is a command substitution.
	q := arrayEnd(work, p+3)
	if q < 0 || q+1 >= len(work) || work[q+1] == ')' {
		return false
	}
	// Both parens become spaces rather than word characters. Blanking them to
	// `_` the way the other rescues do would glue the byte onto the subshell's
	// first and last words — `$((curl url | sh) | tr)` would present its
	// command as `(curl`, which the rules match by name and would not
	// recognise, so a PKGBUILD could hide a network fetch behind this
	// spelling. As whitespace the parens leave every command word exactly as
	// written, at its original offset; what is lost is only the subshell's
	// grouping, so `(a || b) | c` flattens to `a || b | c`. That is a
	// structural change, recorded as such, and it can only widen what a rule
	// sees, never narrow it.
	structural[p+2], structural[q] = true, true
	work[p+2], work[q] = ' ', ' '
	return true
}

// rescueHeredocText handles parse failures inside the body of an unquoted
// here-document. Bash stores the body as text at parse time and only performs
// its expansions when the redirection runs, so `bash -n` accepts a body whose
// backquoted regions are not shell at all — Markdown code fences in a
// user-facing message are the shape seen in the wild. Upstream parses those
// expansions eagerly and fails. Every backquote in the enclosing body is
// blanked to `_` and restored afterwards; the body's `$` expansions stay live.
// A wrong guess about the body's extent is caught downstream: either the
// rewritten file still fails to parse or the bytes do not resurface in the
// body's literal, and the rescue is discarded.
func rescueHeredocText(work []byte, off int, restore map[int]byte) bool {
	for i := 0; i+1 < off; i++ {
		if work[i] != '<' || work[i+1] != '<' {
			continue
		}
		if (i > 0 && work[i-1] == '<') || (i+2 < len(work) && work[i+2] == '<') {
			i++ // <<<: a herestring, not a heredoc
			continue
		}
		j := i + 2
		if j < len(work) && work[j] == '-' {
			j++
		}
		for j < len(work) && (work[j] == ' ' || work[j] == '\t') {
			j++
		}
		ds := j
		for j < len(work) && isIdentByte(work[j]) {
			j++
		}
		if j == ds {
			continue // quoted or exotic delimiter: bash treats the body literally, upstream agrees
		}
		delim := string(work[ds:j])
		nl := bytes.IndexByte(work[j:], '\n')
		if nl < 0 {
			return false
		}
		bodyStart := j + nl + 1
		if off < bodyStart {
			continue
		}
		bodyEnd := -1
		for k := bodyStart; k < len(work); {
			lineEnd := bytes.IndexByte(work[k:], '\n')
			if lineEnd < 0 {
				lineEnd = len(work) - k
			}
			// <<- strips leading tabs from the terminator; plain << does not,
			// but matching both here only widens where the scan stops looking.
			if string(bytes.TrimLeft(work[k:k+lineEnd], "\t")) == delim {
				bodyEnd = k
				break
			}
			k += lineEnd + 1
		}
		if bodyEnd < 0 || off >= bodyEnd {
			continue
		}
		changed := false
		for k := bodyStart; k < bodyEnd; k++ {
			if work[k] != '`' {
				continue
			}
			if _, seen := restore[k]; !seen {
				restore[k] = work[k]
			}
			work[k] = '_'
			changed = true
		}
		return changed
	}
	return false
}

// arrayEnd scans from just inside an array literal's opening paren to its
// closing paren, tracking quoting, escapes, comments, and nested parens. It
// returns -1 when the scan is not confident; the rescue then gives up and the
// original error stands.
func arrayEnd(work []byte, i int) int {
	depth := 1
	for ; i < len(work); i++ {
		switch work[i] {
		case '\\':
			i++
		case '\'':
			for i++; i < len(work); i++ {
				if work[i] == '\'' {
					break
				}
			}
		case '"':
			for i++; i < len(work); i++ {
				if work[i] == '\\' {
					i++
					continue
				}
				if work[i] == '"' {
					break
				}
			}
		case '#':
			// A comment, but only at the start of a word.
			if i == 0 || work[i-1] == ' ' || work[i-1] == '\t' || work[i-1] == '\n' || work[i-1] == '(' {
				for ; i < len(work) && work[i] != '\n'; i++ {
				}
				i--
			}
		case '`':
			return -1 // backquotes nest by escaping; not worth guessing
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// restoreBytes writes the original bytes back into the literals of the
// rescued AST and reports whether every one of them was found. A byte that
// does not resurface became lexical structure instead of text, meaning the
// rewrite was not the spelling-only change it is required to be; the caller
// must then discard the rescue.
func restoreBytes(f *syntax.File, restore map[int]byte) bool {
	if len(restore) == 0 {
		return true
	}
	// Sorted, so each literal finds its own offsets by binary search rather
	// than a sweep of the whole map: a Latin-1 file contributes an entry per
	// undecodable byte, and the sweep made a 200 KiB one take tens of seconds.
	offs := slices.Sorted(maps.Keys(restore))
	done := make([]bool, len(offs))
	patch := func(val string, start int) string {
		i, _ := slices.BinarySearch(offs, start)
		if i == len(offs) || offs[i] >= start+len(val) {
			return val
		}
		b := []byte(val)
		for ; i < len(offs) && offs[i] < start+len(val); i++ {
			b[offs[i]-start] = restore[offs[i]]
			done[i] = true
		}
		return string(b)
	}
	syntax.Walk(f, func(n syntax.Node) bool {
		switch x := n.(type) {
		case *syntax.Lit:
			// A literal spanning an escaped newline stores more bytes than its
			// value; offsets inside it cannot be mapped, so leave it to the
			// final completeness check to reject the rescue.
			if int(x.End().Offset())-int(x.ValuePos.Offset()) == len(x.Value) {
				x.Value = patch(x.Value, int(x.ValuePos.Offset()))
			}
		case *syntax.SglQuoted:
			x.Value = patch(x.Value, int(x.Right.Offset())-len(x.Value))
		case *syntax.Comment:
			x.Text = patch(x.Text, int(x.Hash.Offset())+1)
		}
		return true
	})
	return !slices.Contains(done, false)
}
