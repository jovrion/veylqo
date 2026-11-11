package adapters

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"github.com/jovrion/veylqo/internal/links/domain/link"
)

const (
	// A byte has 256 values. 256 is not divisible by 62, so reducing with
	// modulo would make the low residues more likely. Bytes at or above
	// this limit are discarded (ADR-0006).
	codeAcceptLimit = 256 - (256 % len(link.Base62Alphabet))

	// codeDrawLimit bounds rejection retries for one character. A correct
	// crypto/rand source almost never hits it. A source that only returns
	// rejected bytes must fail instead of spinning on the shorten path.
	codeDrawLimit = 16
)

// CodeGenerator mints a public code. It does not read the database and it
// does not allocate the internal UUID. Collision retries belong to the
// shorten command (ADR-0006).
//
// The alphabet is link.Base62Alphabet. This type does not define another,
// and it does not emit '_' or '-'. Reserved names stay a domain rule.
type CodeGenerator struct {
	src io.Reader
}

// NewCodeGenerator reads entropy from crypto/rand. The length is
// link.RandomCodeLength. The constructor does not panic.
func NewCodeGenerator() *CodeGenerator {
	return &CodeGenerator{src: rand.Reader}
}

// NewCode returns one unbiased base62 code of length 7, or an error if
// the context is done or entropy cannot be read. It does not consult a
// store. A cancelled context is not turned into a code.
func (g *CodeGenerator) NewCode(ctx context.Context) (string, error) {
	if g == nil || g.src == nil {
		return "", errors.New("code generator: nil")
	}
	if ctx == nil {
		return "", errors.New("code generator: nil context")
	}
	out := make([]byte, link.RandomCodeLength)
	for i := range out {
		symbol, err := g.draw(ctx)
		if err != nil {
			return "", err
		}
		out[i] = symbol
	}
	return string(out), nil
}

// draw returns one alphabet byte. Raw values in [codeAcceptLimit, 255]
// are read and discarded so each symbol has the same number of sources.
func (g *CodeGenerator) draw(ctx context.Context) (byte, error) {
	var scratch [1]byte
	for range codeDrawLimit {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		n, err := g.src.Read(scratch[:])
		if n == 1 {
			raw := int(scratch[0])
			if raw < codeAcceptLimit {
				return link.Base62Alphabet[raw%len(link.Base62Alphabet)], nil
			}
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("code generator: read entropy: %w", err)
		}
		return 0, errors.New("code generator: empty read")
	}
	return 0, errors.New("code generator: rejected entropy")
}
