package llm

import (
	"errors"
	"fmt"

	"github.com/hybridgroup/yzma/pkg/llama"
)

// ErrNoLogits is returned by [Context.Logits] for a position the last
// [Context.Decode] did not evaluate.
var ErrNoLogits = errors.New("no logits for that position")

// Token is a model's vocabulary index.
type Token = llama.Token

// Model is a loaded GGUF model. [Init] must have succeeded first.
type Model struct {
	model  llama.Model
	vocab  llama.Vocab
	nVocab int
	addBOS bool
}

// Open loads the GGUF model at path.
func Open(path string) (*Model, error) {
	model, err := llama.ModelLoadFromFile(path, llama.ModelDefaultParams())
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", path, err)
	}
	vocab := llama.ModelGetVocab(model)
	return &Model{
		model:  model,
		vocab:  vocab,
		nVocab: int(llama.VocabNTokens(vocab)),
		addBOS: llama.VocabGetAddBOS(vocab),
	}, nil
}

// Close frees the model. Contexts made from it must be closed first.
func (m *Model) Close() error {
	return llama.ModelFree(m.model)
}

// VocabSize is the length of every logits slice the model produces.
func (m *Model) VocabSize() int {
	return m.nVocab
}

// Tokenize splits text into tokens, starting with the model's BOS token when
// it expects one. BOS reports whether it did.
func (m *Model) Tokenize(text string) (tokens []Token, bos bool) {
	tokens = llama.Tokenize(m.vocab, text, false, false)
	if m.addBOS {
		tokens = append([]Token{llama.VocabBOS(m.vocab)}, tokens...)
	}
	return tokens, m.addBOS
}

// Piece is the text of a single token. A character that spans several tokens
// is split between their pieces, so a piece need not be valid UTF-8; the
// pieces of a sequence concatenate back to its text.
func (m *Model) Piece(tok Token) string {
	buf := make([]byte, 64)
	n := llama.TokenToPiece(m.vocab, tok, buf, 0, false)
	if n < 0 {
		buf = make([]byte, -n)
		n = llama.TokenToPiece(m.vocab, tok, buf, 0, false)
	}
	return string(buf[:max(n, 0)])
}

// Context evaluates token sequences of up to its size with one model. It is
// not safe for concurrent use.
type Context struct {
	model *Model
	batch llama.Batch
	ctx   llama.Context
	size  int
	n     int // tokens in the last Decode
}

// NewContext makes a context that holds size tokens and evaluates them batch
// at a time.
func (m *Model) NewContext(size, batch int) (*Context, error) {
	params := llama.ContextDefaultParams()
	params.NCtx = uint32(size)
	params.NBatch = uint32(batch)
	params.NUbatch = uint32(batch)
	ctx, err := llama.InitFromModel(m.model, params)
	if err != nil {
		return nil, err
	}
	return &Context{
		model: m,
		batch: llama.BatchInit(int32(batch), 0, 1),
		ctx:   ctx,
		size:  size,
	}, nil
}

// Close frees the context.
func (c *Context) Close() error {
	_ = llama.BatchFree(c.batch)
	return llama.Free(c.ctx)
}

// Size is the number of tokens the context holds.
func (c *Context) Size() int {
	return c.size
}

// Reset forgets every evaluated token, so the next [Context.Decode] starts a
// new sequence at position 0.
func (c *Context) Reset() error {
	mem, err := llama.GetMemory(c.ctx)
	if err != nil {
		return err
	}
	c.n = 0
	return llama.MemoryClear(mem, true)
}

// Decode evaluates tokens, which continue the sequence at position pos, and
// keeps the logits of every one. At most the batch size given to
// [Model.NewContext] fit in one call.
func (c *Context) Decode(tokens []Token, pos int) error {
	if err := c.batch.Clear(); err != nil {
		return err
	}
	for i, tok := range tokens {
		if err := c.batch.Add(tok, llama.Pos(pos+i), []llama.SeqId{0}, true); err != nil {
			return err
		}
	}
	ret, err := llama.Decode(c.ctx, c.batch)
	if err != nil {
		return err
	}
	if ret != 0 {
		return fmt.Errorf("llama_decode returned %d", ret)
	}
	c.n = len(tokens)
	return nil
}

// Logits are the model's unnormalized scores, one per vocabulary entry, for
// the token that follows the ith token of the last [Context.Decode]. The
// slice is llama.cpp's memory: it is overwritten by the next Decode.
func (c *Context) Logits(i int) ([]float32, error) {
	if i < 0 || i >= c.n {
		return nil, fmt.Errorf("%w: %d of %d", ErrNoLogits, i, c.n)
	}
	logits, err := llama.GetLogitsIth(c.ctx, int32(i), c.model.nVocab)
	if err != nil {
		return nil, err
	}
	if logits == nil {
		return nil, fmt.Errorf("%w: %d", ErrNoLogits, i)
	}
	return logits, nil
}
