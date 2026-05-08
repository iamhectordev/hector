package processor

import (
	"context"
	"fmt"
	"io"
	"os"
)

// Processor handles inbound text independent of input surface.
type Processor struct {
	out io.Writer
}

// New returns a processor that writes handled lines to out, defaulting to stdout.
func New(out io.Writer) *Processor {
	if out == nil {
		out = os.Stdout
	}
	return &Processor{out: out}
}

// Handle prints text as a line (behavior used by the chat echo path).
func (p *Processor) Handle(_ context.Context, text string) error {
	_, err := fmt.Fprintln(p.out, text)
	return err
}
