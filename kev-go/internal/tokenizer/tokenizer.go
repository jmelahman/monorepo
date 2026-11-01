// Package tokenizer encodes text the way Hugging Face tokenizers does for
// Qwen's tokenizer.json: added tokens split out first, the rest NFC-normalized,
// cut into pieces by a GPT-2 style pattern, mapped to byte-level characters and
// merged by BPE.
package tokenizer

import (
	"container/heap"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// A Tokenizer is safe for concurrent use.
type Tokenizer struct {
	vocab map[string]int32
	// merges maps a pair of ids to its rank and the id it merges into.
	merges map[[2]int32]merge
	// bytes is the id of each byte's single-character token.
	bytes [256]int32
	// added holds the added tokens, longest first within a first byte.
	added map[byte][]addedToken
}

type merge struct{ rank, id int32 }

type addedToken struct {
	content string
	id      int32
}

type file struct {
	AddedTokens []struct {
		ID         int32  `json:"id"`
		Content    string `json:"content"`
		Normalized bool   `json:"normalized"`
		LStrip     bool   `json:"lstrip"`
		RStrip     bool   `json:"rstrip"`
		SingleWord bool   `json:"single_word"`
	} `json:"added_tokens"`
	Normalizer struct {
		Type string `json:"type"`
	} `json:"normalizer"`
	Model struct {
		Type   string           `json:"type"`
		Vocab  map[string]int32 `json:"vocab"`
		Merges [][2]string      `json:"merges"`
	} `json:"model"`
}

// Open loads a tokenizer.json. Anything but the shape this package implements
// is refused, since encoding it differently would quietly change every answer.
func Open(path string) (*Tokenizer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if f.Model.Type != "BPE" || f.Normalizer.Type != "NFC" {
		return nil, fmt.Errorf("%s: unsupported tokenizer: model %q, normalizer %q", path, f.Model.Type, f.Normalizer.Type)
	}
	t := &Tokenizer{
		vocab:  f.Model.Vocab,
		merges: make(map[[2]int32]merge, len(f.Model.Merges)),
		added:  map[byte][]addedToken{},
	}
	for b := range 256 {
		id, ok := t.vocab[string(byteChars[b])]
		if !ok {
			return nil, fmt.Errorf("%s: vocab has no token for byte %d", path, b)
		}
		t.bytes[b] = id
	}
	for rank, m := range f.Model.Merges {
		a, aok := t.vocab[m[0]]
		b, bok := t.vocab[m[1]]
		id, idok := t.vocab[m[0]+m[1]]
		if !aok || !bok || !idok {
			return nil, fmt.Errorf("%s: merge %q %q isn't in the vocab", path, m[0], m[1])
		}
		t.merges[[2]int32{a, b}] = merge{int32(rank), id}
	}
	for _, a := range f.AddedTokens {
		if a.Normalized || a.LStrip || a.RStrip || a.SingleWord || a.Content == "" {
			return nil, fmt.Errorf("%s: unsupported added token %q", path, a.Content)
		}
		t.vocab[a.Content] = a.ID
		t.added[a.Content[0]] = append(t.added[a.Content[0]], addedToken{a.Content, a.ID})
	}
	for _, list := range t.added {
		sort.Slice(list, func(i, j int) bool { return len(list[i].content) > len(list[j].content) })
	}
	return t, nil
}

// ID returns the id of a token, as convert_tokens_to_ids does.
func (t *Tokenizer) ID(token string) (int32, bool) {
	id, ok := t.vocab[token]
	return id, ok
}

// Encode returns the ids of text, with no special tokens added around it.
func (t *Tokenizer) Encode(text string) []int32 {
	var ids []int32
	start := 0
	for i := 0; i < len(text); {
		a, ok := t.addedAt(text, i)
		if !ok {
			i++
			continue
		}
		ids = t.encodePlain(ids, text[start:i])
		ids = append(ids, a.id)
		i += len(a.content)
		start = i
	}
	return t.encodePlain(ids, text[start:])
}

func (t *Tokenizer) addedAt(text string, i int) (addedToken, bool) {
	for _, a := range t.added[text[i]] {
		if strings.HasPrefix(text[i:], a.content) {
			return a, true
		}
	}
	return addedToken{}, false
}

func (t *Tokenizer) encodePlain(ids []int32, text string) []int32 {
	if text == "" {
		return ids
	}
	text = norm.NFC.String(strings.ToValidUTF8(text, "\uFFFD"))
	for len(text) > 0 {
		n := nextPiece(text)
		ids = t.bpe(ids, text[:n])
		text = text[n:]
	}
	return ids
}

// nextPiece returns the length of the first match of
//
//	(?i:'s|'t|'re|'ve|'m|'ll|'d)|[^\r\n\p{L}\p{N}]?\p{L}+|\p{N}| ?[^\s\p{L}\p{N}]+[\r\n]*|\s*[\r\n]+|\s+(?!\S)|\s+
//
// in text. It's written out because Go's regexp has no lookahead. Every rune
// matches some alternative, so the result is never zero.
func nextPiece(s string) int {
	r, w := utf8.DecodeRuneInString(s)
	if r == '\'' {
		if n := contraction(s[w:]); n > 0 {
			return w + n
		}
	}
	if isLetter(r) {
		return w + letters(s[w:])
	}
	if !isNewline(r) && !isNumber(r) {
		if n := letters(s[w:]); n > 0 {
			return w + n
		}
	}
	if isNumber(r) {
		return w
	}
	sym := s
	symStart := 0
	if r == ' ' {
		symStart = w
		sym = s[w:]
	}
	if n := symbols(sym); n > 0 {
		end := symStart + n
		for end < len(s) && (s[end] == '\r' || s[end] == '\n') {
			end++
		}
		return end
	}
	// Whitespace from here: r is neither letter, number nor symbol.
	end, lastNewline, last := 0, 0, 0
	for end < len(s) {
		c, cw := utf8.DecodeRuneInString(s[end:])
		if !unicode.IsSpace(c) {
			break
		}
		last = end
		end += cw
		if isNewline(c) {
			lastNewline = end
		}
	}
	if lastNewline > 0 {
		return lastNewline
	}
	if end < len(s) && last > 0 {
		// \s+(?!\S) gives back the space before a non-space.
		return last
	}
	return end
}

// contraction matches what follows the apostrophe of 's 't 're 've 'm 'll 'd,
// in any case. U+017F folds to s under Unicode case-insensitivity.
func contraction(s string) int {
	if strings.HasPrefix(s, "ſ") {
		return len("ſ")
	}
	if len(s) == 0 {
		return 0
	}
	switch s[0] | 0x20 {
	case 's', 't', 'm', 'd':
		return 1
	}
	if len(s) < 2 {
		return 0
	}
	a, b := s[0]|0x20, s[1]|0x20
	if a == 'r' && b == 'e' || a == 'v' && b == 'e' || a == 'l' && b == 'l' {
		return 2
	}
	return 0
}

func letters(s string) int {
	n := 0
	for n < len(s) {
		r, w := utf8.DecodeRuneInString(s[n:])
		if !isLetter(r) {
			break
		}
		n += w
	}
	return n
}

func symbols(s string) int {
	n := 0
	for n < len(s) {
		r, w := utf8.DecodeRuneInString(s[n:])
		if unicode.IsSpace(r) || isLetter(r) || isNumber(r) {
			break
		}
		n += w
	}
	return n
}

func isLetter(r rune) bool  { return unicode.IsLetter(r) }
func isNumber(r rune) bool  { return unicode.IsNumber(r) }
func isNewline(r rune) bool { return r == '\r' || r == '\n' }

// bpe appends the ids of one piece, merging its lowest-ranked pair first and
// leftmost first among equals. A heap keeps a long piece from going quadratic.
func (t *Tokenizer) bpe(ids []int32, piece string) []int32 {
	if id, ok := t.vocab[byteLevel(piece)]; ok {
		return append(ids, id)
	}
	syms := make([]symbol, len(piece))
	for i := range syms {
		syms[i] = symbol{id: t.bytes[piece[i]], prev: i - 1, next: i + 1}
	}
	syms[len(syms)-1].next = -1
	var q pairHeap
	for i := range len(syms) - 1 {
		if m, ok := t.merges[[2]int32{syms[i].id, syms[i+1].id}]; ok {
			q = append(q, pair{m.rank, i, syms[i].id, syms[i+1].id, m.id})
		}
	}
	heap.Init(&q)
	for q.Len() > 0 {
		p := heap.Pop(&q).(pair)
		l := &syms[p.left]
		if l.dead || l.id != p.a || l.next < 0 || syms[l.next].id != p.b {
			continue
		}
		syms[l.next].dead = true
		l.id = p.id
		l.next = syms[l.next].next
		if l.next >= 0 {
			syms[l.next].prev = p.left
			if m, ok := t.merges[[2]int32{l.id, syms[l.next].id}]; ok {
				heap.Push(&q, pair{m.rank, p.left, l.id, syms[l.next].id, m.id})
			}
		}
		if l.prev >= 0 {
			if m, ok := t.merges[[2]int32{syms[l.prev].id, l.id}]; ok {
				heap.Push(&q, pair{m.rank, l.prev, syms[l.prev].id, l.id, m.id})
			}
		}
	}
	for i := 0; i >= 0; i = syms[i].next {
		ids = append(ids, syms[i].id)
	}
	return ids
}

type symbol struct {
	id         int32
	prev, next int
	dead       bool
}

type pair struct {
	rank     int32
	left     int
	a, b, id int32
}

type pairHeap []pair

func (h pairHeap) Len() int { return len(h) }
func (h pairHeap) Less(i, j int) bool {
	if h[i].rank != h[j].rank {
		return h[i].rank < h[j].rank
	}
	return h[i].left < h[j].left
}
func (h pairHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *pairHeap) Push(x any)   { *h = append(*h, x.(pair)) }
func (h *pairHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}

// byteChars is GPT-2's byte-to-character table: printable bytes stand for
// themselves, the rest take characters from U+0100 on.
var byteChars = func() (out [256]rune) {
	next := rune(256)
	for b := range 256 {
		switch {
		case b >= '!' && b <= '~', b >= 0xA1 && b <= 0xAC, b >= 0xAE && b <= 0xFF:
			out[b] = rune(b)
		default:
			out[b] = next
			next++
		}
	}
	return out
}()

func byteLevel(s string) string {
	var b strings.Builder
	b.Grow(len(s) * 2)
	for i := range len(s) {
		b.WriteRune(byteChars[s[i]])
	}
	return b.String()
}
