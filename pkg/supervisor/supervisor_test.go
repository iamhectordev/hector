package supervisor_test

import (
	"context"
	"errors"
	"os"
	"runtime"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/iamhectordev/hector/pkg/supervisor"
	"github.com/stretchr/testify/require"
)

func TestNew_InvalidStopTimeout(t *testing.T) {
	t.Parallel()
	_, err := supervisor.New([]supervisor.Module{
		exitModule{name: "noop"},
	}, supervisor.WithStopTimeout(-1*time.Second))
	require.Error(t, err)
}

func TestNew_NoModules(t *testing.T) {
	t.Parallel()
	_, err := supervisor.New(nil, supervisor.WithStopTimeout(50*time.Millisecond))
	require.Error(t, err)
	require.Contains(t, err.Error(), "at least one module")
}

func TestNew_SignalHandlingConflictsWithSignalChan(t *testing.T) {
	t.Parallel()
	_, err := supervisor.New([]supervisor.Module{exitModule{name: "noop"}},
		supervisor.WithSignalHandling(),
		supervisor.WithSignalChan(make(chan os.Signal)),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be used together")
}

func TestRun_ModuleError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, err := supervisor.New([]supervisor.Module{
			errModule{name: "bad"},
		},
			supervisor.WithSignalChan(make(chan os.Signal)),
			supervisor.WithStopTimeout(50*time.Millisecond),
		)
		require.NoError(t, err)

		rep := s.Run(t.Context())
		synctest.Wait()

		require.Equal(t, supervisor.ReasonModuleError, rep.Reason)
		require.Equal(t, "bad", rep.TriggerModule)
		require.EqualError(t, rep.Cause, "boom")
	})
}

func TestRun_ContextSignalCause(t *testing.T) {
	t.Parallel()
	s, err := supervisor.New([]supervisor.Module{
		blockUntilCanceled{name: "block", started: make(chan struct{}, 1)},
	}, supervisor.WithStopTimeout(50*time.Millisecond))
	require.NoError(t, err)

	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(supervisor.SignalCause{Signal: os.Interrupt})

	rep := s.Run(ctx)
	require.Equal(t, supervisor.ReasonSignal, rep.Reason)
	require.Equal(t, os.Interrupt, rep.Signal)
	require.Error(t, rep.Cause)
}

func TestRun_ModulePanic(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, err := supervisor.New([]supervisor.Module{
			panicModule{name: "panic"},
		},
			supervisor.WithSignalChan(make(chan os.Signal)),
			supervisor.WithStopTimeout(50*time.Millisecond),
		)
		require.NoError(t, err)

		rep := s.Run(t.Context())
		synctest.Wait()

		require.Equal(t, supervisor.ReasonModulePanic, rep.Reason)
		require.Equal(t, "panic", rep.TriggerModule)
		require.Equal(t, "oops", rep.PanicValue)
	})
}

func TestRun_ModuleStopped(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, err := supervisor.New([]supervisor.Module{
			exitModule{name: "exit"},
		},
			supervisor.WithSignalChan(make(chan os.Signal)),
			supervisor.WithStopTimeout(50*time.Millisecond),
		)
		require.NoError(t, err)

		rep := s.Run(t.Context())
		synctest.Wait()

		require.Equal(t, supervisor.ReasonModuleStopped, rep.Reason)
		require.Equal(t, "exit", rep.TriggerModule)
	})
}

func TestRun_SignalBeforeModuleExit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sig := make(chan os.Signal, 1)
		started := make(chan struct{}, 1)

		s, err := supervisor.New([]supervisor.Module{
			blockUntilCanceled{name: "block", started: started},
		},
			supervisor.WithSignalChan(sig),
			supervisor.WithStopTimeout(50*time.Millisecond),
		)
		require.NoError(t, err)

		done := make(chan supervisor.Report, 1)
		go func() {
			done <- s.Run(t.Context())
		}()

		<-started
		sig <- os.Interrupt
		rep := <-done

		synctest.Wait()

		require.Equal(t, supervisor.ReasonSignal, rep.Reason)
		require.Nil(t, rep.StopErrors)
	})
}

func TestRun_StopRecordsDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sig := make(chan os.Signal, 1)
		s, err := supervisor.New([]supervisor.Module{
			slowStopModule{name: "slow"},
		},
			supervisor.WithSignalChan(sig),
			supervisor.WithStopTimeout(20*time.Millisecond),
		)
		require.NoError(t, err)

		go func() { sig <- os.Interrupt }()
		rep := s.Run(t.Context())
		synctest.Wait()

		require.Equal(t, supervisor.ReasonSignal, rep.Reason)
		require.Contains(t, rep.StopErrors, "slow")
		require.ErrorIs(t, rep.StopErrors["slow"], context.DeadlineExceeded)
	})
}

type errModule struct{ name string }

func (m errModule) Name() string                    { return m.name }
func (m errModule) Start(ctx context.Context) error { return errors.New("boom") }
func (m errModule) Stop(ctx context.Context) error  { return nil }

type panicModule struct{ name string }

func (m panicModule) Name() string                    { return m.name }
func (m panicModule) Start(ctx context.Context) error { panic("oops") }
func (m panicModule) Stop(ctx context.Context) error  { return nil }

type exitModule struct{ name string }

func (m exitModule) Name() string                    { return m.name }
func (m exitModule) Start(ctx context.Context) error { return nil }
func (m exitModule) Stop(ctx context.Context) error  { return nil }

type blockUntilCanceled struct {
	name    string
	started chan struct{}
}

func (m blockUntilCanceled) Name() string { return m.name }

func (m blockUntilCanceled) Start(ctx context.Context) error {
	select {
	case m.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return nil
}

func (m blockUntilCanceled) Stop(ctx context.Context) error { return nil }

type slowStopModule struct{ name string }

func (m slowStopModule) Name() string { return m.name }

func (m slowStopModule) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (m slowStopModule) Stop(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestReport_Err_NilOnCleanSignal(t *testing.T) {
	t.Parallel()
	r := supervisor.Report{Reason: supervisor.ReasonSignal}
	require.NoError(t, r.Err())
}

func TestReport_Err_ModuleError(t *testing.T) {
	t.Parallel()
	cause := errors.New("boom")
	r := supervisor.Report{Reason: supervisor.ReasonModuleError, Cause: cause}
	require.ErrorIs(t, r.Err(), cause)
}

func TestReport_Err_JoinsStopErrors(t *testing.T) {
	t.Parallel()
	stopErr := errors.New("stop failed")
	r := supervisor.Report{
		Reason:     supervisor.ReasonSignal,
		StopErrors: map[string]error{"agent": stopErr},
	}
	err := r.Err()
	require.ErrorIs(t, err, stopErr)
}

func TestReport_Err_ContextCanceledIncludesCause(t *testing.T) {
	t.Parallel()
	cause := context.Canceled
	r := supervisor.Report{
		Reason: supervisor.ReasonContextCanceled,
		Cause:  cause,
	}
	require.ErrorIs(t, r.Err(), cause)
}

func TestNotifyContext_StopRaceDoesNotLoseSignalCause(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("uses SIGUSR1")
	}

	for i := 0; i < 200; i++ {
		ctx, stop := supervisor.NotifyContext(context.Background(), syscall.SIGUSR1)

		require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGUSR1))
		<-ctx.Done()

		stopDone := make(chan struct{})
		go func() {
			stop()
			close(stopDone)
		}()

		<-stopDone

		cause := context.Cause(ctx)
		var signalCause supervisor.SignalCause
		require.True(t, errors.As(cause, &signalCause), "signal cause must be preserved after stop()")
	}
}
