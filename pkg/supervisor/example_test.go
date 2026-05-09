package supervisor_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/iamhectordev/hector/pkg/supervisor"
)

type exampleModule struct {
	name string
}

func (m exampleModule) Name() string                      { return m.name }
func (m exampleModule) Init(ctx context.Context) error    { return nil }
func (m exampleModule) Start(ctx context.Context) error   { <-ctx.Done(); return nil }
func (m exampleModule) Stop(ctx context.Context) error    { return nil }

func Example_minimal() {
	s, err := supervisor.New([]supervisor.Module{
		exampleModule{name: "api"},
		exampleModule{name: "worker"},
	})
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_ = s.Run(ctx)
}

func Example_withLogger() {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := supervisor.New(
		[]supervisor.Module{exampleModule{name: "api"}},
		supervisor.WithLogger(logger),
		supervisor.WithStopTimeout(2*time.Second),
	)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_ = s.Run(ctx)
}

func Example_withSignalHandling() {
	s, err := supervisor.New(
		[]supervisor.Module{exampleModule{name: "api"}},
		supervisor.WithSignalHandling(os.Interrupt),
	)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// In a real process this returns on Ctrl+C.
	cancel()
	_ = s.Run(ctx)
}
