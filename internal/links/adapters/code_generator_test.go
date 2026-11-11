package adapters

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jovrion/veylqo/internal/links/app/command"
	"github.com/jovrion/veylqo/internal/links/domain/link"
)

// Adapters match the command method set. The command package does not
// import this one (ADR-0006).
var _ command.CodeGenerator = (*CodeGenerator)(nil)

func TestCodeGeneratorThousandCodes(t *testing.T) {
	if link.RandomCodeLength != 7 {
		t.Fatalf("RandomCodeLength = %d, want 7", link.RandomCodeLength)
	}
	g := NewCodeGenerator()
	seen := make(map[string]struct{}, 1000)
	for i := range 1000 {
		code, err := g.NewCode(context.Background())
		if err != nil {
			t.Fatalf("code %d: %v", i, err)
		}
		if len(code) != 7 {
			t.Fatalf("code %d length = %d, want 7", i, len(code))
		}
		if strings.ContainsAny(code, "_-") {
			t.Fatalf("code %d %q is not base62", i, code)
		}
		for j := 0; j < len(code); j++ {
			if strings.IndexByte(link.Base62Alphabet, code[j]) < 0 {
				t.Fatalf("code %d %q has %q outside base62", i, code, code[j])
			}
		}
		if _, err := link.NewCode(code); err != nil {
			var invalid *link.InvalidCodeError
			if errors.As(err, &invalid) && invalid.Reason == "reserved" {
				continue
			}
			t.Fatalf("code %d %q: %v", i, code, err)
		}
		seen[code] = struct{}{}
	}
	if len(seen) < 2 {
		t.Fatalf("unique codes = %d, want a trivial non-collision", len(seen))
	}
}

func TestCodeGeneratorRejectionSampling(t *testing.T) {
	// Two rejected bytes, then one accepted byte, for each of 7 symbols.
	// 0, 62, 124, and 186 are four unbiased sources of alphabet[0].
	// 248 and 255 must be discarded, not reduced onto the alphabet.
	accepted := []byte{0, 62, 124, 186, 61, 123, 247}
	var script []byte
	for _, raw := range accepted {
		script = append(script, 255, 248, raw)
	}
	src := &scriptReader{buf: script}
	g := &CodeGenerator{src: src}
	got, err := g.NewCode(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := string([]byte{
		link.Base62Alphabet[0],
		link.Base62Alphabet[0],
		link.Base62Alphabet[0],
		link.Base62Alphabet[0],
		link.Base62Alphabet[61],
		link.Base62Alphabet[61],
		link.Base62Alphabet[61],
	})
	if got != want {
		t.Fatalf("NewCode = %q, want %q", got, want)
	}
	if src.n != len(script) {
		t.Fatalf("bytes read = %d, want %d including rejected values", src.n, len(script))
	}
}

func TestCodeGeneratorStopsWhenEntropyIsOnlyRejected(t *testing.T) {
	g := &CodeGenerator{src: constReader(255)}
	if _, err := g.NewCode(context.Background()); err == nil {
		t.Fatal("rejected entropy was accepted")
	}
}

func TestCodeGeneratorRejectsContext(t *testing.T) {
	g := &CodeGenerator{src: failReader{}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := g.NewCode(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled NewCode = %v, want context.Canceled", err)
	}
	var nilCtx context.Context
	if _, err := g.NewCode(nilCtx); err == nil {
		t.Fatal("nil context was accepted")
	}
	if _, err := (*CodeGenerator)(nil).NewCode(context.Background()); err == nil {
		t.Fatal("nil generator was accepted")
	}
}

func TestCodeGeneratorReadError(t *testing.T) {
	g := &CodeGenerator{src: errReader{err: io.ErrUnexpectedEOF}}
	_, err := g.NewCode(context.Background())
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("NewCode = %v, want unexpected EOF", err)
	}
}

type scriptReader struct {
	buf []byte
	n   int
}

func (s *scriptReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if s.n >= len(s.buf) {
		return 0, io.EOF
	}
	p[0] = s.buf[s.n]
	s.n++
	return 1, nil
}

type constReader byte

func (c constReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(c)
	}
	if len(p) == 0 {
		return 0, nil
	}
	return len(p), nil
}

type failReader struct{}

func (failReader) Read([]byte) (int, error) {
	return 0, errors.New("code generator test: read after rejected context")
}

type errReader struct {
	err error
}

func (r errReader) Read([]byte) (int, error) {
	return 0, r.err
}
