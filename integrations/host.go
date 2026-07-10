package integrations

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/iamhectordev/hector/pkg/telem"
)

// Host wraps an [Integration] as a [supervisor.Module].
type Host struct {
	i Integration
}

// NewHost creates a Host that adapts i to the module lifecycle.
// It returns an error if i is nil or its Name is empty.
func NewHost(i Integration) (*Host, error) {
	if i == nil {
		return nil, errors.New("integrations: nil integration")
	}
	if i.Name() == "" {
		return nil, errors.New("integrations: empty integration name")
	}
	return &Host{i: i}, nil
}

func (h *Host) Name() string { return "integration." + h.i.Name() }

func (h *Host) log(ctx context.Context) telem.ContextLogger {
	return telem.Logger(ctx).With(
		telem.String("component", "module"),
		telem.String("module", h.Name()),
	)
}

func (h *Host) Init(ctx context.Context) error {
	if init, ok := h.i.(Initializer); ok {
		h.log(ctx).InfoContext(ctx, "initializing")
		return init.Init(ctx)
	}
	return nil
}

func (h *Host) Start(ctx context.Context) error {
	h.log(ctx).InfoContext(ctx, "starting")
	if src, ok := h.i.(EventSource); ok {
		err := src.Run(ctx)
		if err != nil && ctx.Err() != nil {
			return nil
		}
		if err != nil {
			return fmt.Errorf("integration %s: %w", h.i.Name(), err)
		}
		return nil
	}
	<-ctx.Done()
	return nil
}

func (h *Host) Stop(ctx context.Context) error {
	if c, ok := h.i.(io.Closer); ok {
		h.log(ctx).InfoContext(ctx, "stopping")
		return c.Close()
	}
	return nil
}
