package supervisor_test

import (
	"context"
	"errors"
	"os"
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

func TestNew_DuplicatePreStopHookNames(t *testing.T) {
	t.Parallel()
	_, err := supervisor.New([]supervisor.Module{exitModule{name: "noop"}},
		supervisor.WithPreStopHook("same", func(context.Context) error { return nil }),
		supervisor.WithPreStopHook("same", func(context.Context) error { return nil }),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate pre-stop hook name")
}

func TestNew_DuplicatePostInitHookNames(t *testing.T) {
	t.Parallel()
	_, err := supervisor.New([]supervisor.Module{exitModule{name: "noop"}},
		supervisor.WithPostInitHook("same", func(context.Context) error { return nil }),
		supervisor.WithPostInitHook("same", func(context.Context) error { return nil }),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate post-init hook name")
}

func TestNew_NilHookFuncRejected(t *testing.T) {
	t.Parallel()
	_, err := supervisor.New([]supervisor.Module{exitModule{name: "noop"}},
		supervisor.WithPostInitHook("bus.start", nil),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "function cannot be nil")
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

func TestRun_InitError(t *testing.T) {
	t.Parallel()
	s, err := supervisor.New([]supervisor.Module{
		initErrModule{name: "bad-init"},
	}, supervisor.WithStopTimeout(50*time.Millisecond))
	require.NoError(t, err)

	rep := s.Run(t.Context())

	require.Equal(t, supervisor.ReasonInitError, rep.Reason)
	require.ErrorContains(t, rep.Cause, "init boom")
	require.ErrorContains(t, rep.Err(), "init boom")
}

func TestRun_PostInitHookRunsAfterInitBeforeStart(t *testing.T) {
	sig := make(chan os.Signal, 1)
	order := make(chan string, 5)
	s, err := supervisor.New([]supervisor.Module{
		lifecycleModule{
			name:    "one",
			onInit:  func() { order <- "one.init" },
			onStart: func() { order <- "one.start" },
		},
		lifecycleModule{
			name:    "two",
			onInit:  func() { order <- "two.init" },
			onStart: func() { order <- "two.start" },
		},
	},
		supervisor.WithSignalChan(sig),
		supervisor.WithStopTimeout(50*time.Millisecond),
		supervisor.WithPostInitHook("bus.start", func(context.Context) error {
			order <- "post.bus.start"
			return nil
		}),
	)
	require.NoError(t, err)

	done := make(chan supervisor.Report, 1)
	go func() { done <- s.Run(t.Context()) }()

	require.Equal(t, "one.init", <-order)
	require.Equal(t, "two.init", <-order)
	require.Equal(t, "post.bus.start", <-order)
	require.Contains(t, []string{"one.start", "two.start"}, <-order)

	sig <- os.Interrupt
	rep := <-done
	require.Equal(t, supervisor.ReasonSignal, rep.Reason)
}

func TestRun_PostInitHookErrorPreventsStart(t *testing.T) {
	postErr := errors.New("bus failed")
	started := make(chan struct{}, 1)
	s, err := supervisor.New([]supervisor.Module{
		lifecycleModule{
			name:    "mod",
			onStart: func() { started <- struct{}{} },
		},
	},
		supervisor.WithStopTimeout(50*time.Millisecond),
		supervisor.WithPostInitHook("bus.start", func(context.Context) error {
			return postErr
		}),
	)
	require.NoError(t, err)

	rep := s.Run(t.Context())

	require.Equal(t, supervisor.ReasonInitError, rep.Reason)
	require.ErrorIs(t, rep.Cause, postErr)
	require.ErrorContains(t, rep.Cause, "post-init hook")
	require.Empty(t, started)
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

func TestRun_HooksExecuteInOrderAroundModuleStop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sig := make(chan os.Signal, 1)
		started := make(chan struct{}, 1)
		order := make(chan string, 3)

		mod := orderedModule{
			name:    "mod",
			started: started,
			onStop: func() {
				order <- "module.stop"
			},
		}
		s, err := supervisor.New([]supervisor.Module{mod},
			supervisor.WithSignalChan(sig),
			supervisor.WithStopTimeout(50*time.Millisecond),
			supervisor.WithPreStopHook("bus.drain", func(context.Context) error {
				order <- "pre.bus.drain"
				return nil
			}),
			supervisor.WithPostStopHook("db.close", func(context.Context) error {
				order <- "post.db.close"
				return nil
			}),
		)
		require.NoError(t, err)

		done := make(chan supervisor.Report, 1)
		go func() { done <- s.Run(t.Context()) }()

		<-started
		sig <- os.Interrupt
		rep := <-done
		synctest.Wait()

		require.Equal(t, supervisor.ReasonSignal, rep.Reason)
		require.Equal(t, "pre.bus.drain", <-order)
		require.Equal(t, "module.stop", <-order)
		require.Equal(t, "post.db.close", <-order)
	})
}

func TestRun_HookErrorsAreReported(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sig := make(chan os.Signal, 1)
		preErr := errors.New("drain failed")
		postErr := errors.New("close failed")

		s, err := supervisor.New([]supervisor.Module{
			blockUntilCanceled{name: "block", started: make(chan struct{}, 1)},
		},
			supervisor.WithSignalChan(sig),
			supervisor.WithStopTimeout(50*time.Millisecond),
			supervisor.WithPreStopHook("bus.drain", func(context.Context) error { return preErr }),
			supervisor.WithPostStopHook("db.close", func(context.Context) error { return postErr }),
		)
		require.NoError(t, err)

		go func() { sig <- os.Interrupt }()
		rep := s.Run(t.Context())
		synctest.Wait()

		require.Equal(t, supervisor.ReasonSignal, rep.Reason)
		require.ErrorIs(t, rep.PreStopErrors["bus.drain"], preErr)
		require.ErrorIs(t, rep.PostStopErrors["db.close"], postErr)
		require.ErrorIs(t, rep.Err(), preErr)
		require.ErrorIs(t, rep.Err(), postErr)
	})
}

func TestRun_PreStopHookPanicCaptured(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sig := make(chan os.Signal, 1)
		s, err := supervisor.New([]supervisor.Module{
			blockUntilCanceled{name: "block", started: make(chan struct{}, 1)},
		},
			supervisor.WithSignalChan(sig),
			supervisor.WithStopTimeout(50*time.Millisecond),
			supervisor.WithPreStopHook("panic.hook", func(context.Context) error {
				panic("boom")
			}),
		)
		require.NoError(t, err)

		go func() { sig <- os.Interrupt }()
		rep := s.Run(t.Context())
		synctest.Wait()

		require.Equal(t, supervisor.ReasonSignal, rep.Reason)
		require.Error(t, rep.PreStopErrors["panic.hook"])
		require.Contains(t, rep.PreStopErrors["panic.hook"].Error(), "panicked")
	})
}

type errModule struct{ name string }

func (m errModule) Name() string                    { return m.name }
func (m errModule) Init(ctx context.Context) error  { return nil }
func (m errModule) Start(ctx context.Context) error { return errors.New("boom") }
func (m errModule) Stop(ctx context.Context) error  { return nil }

type initErrModule struct{ name string }

func (m initErrModule) Name() string                    { return m.name }
func (m initErrModule) Init(context.Context) error      { return errors.New("init boom") }
func (m initErrModule) Start(ctx context.Context) error { return nil }
func (m initErrModule) Stop(ctx context.Context) error  { return nil }

type panicModule struct{ name string }

func (m panicModule) Name() string                    { return m.name }
func (m panicModule) Init(ctx context.Context) error  { return nil }
func (m panicModule) Start(ctx context.Context) error { panic("oops") }
func (m panicModule) Stop(ctx context.Context) error  { return nil }

type exitModule struct{ name string }

func (m exitModule) Name() string                    { return m.name }
func (m exitModule) Init(ctx context.Context) error  { return nil }
func (m exitModule) Start(ctx context.Context) error { return nil }
func (m exitModule) Stop(ctx context.Context) error  { return nil }

type lifecycleModule struct {
	name    string
	onInit  func()
	onStart func()
	onStop  func()
}

func (m lifecycleModule) Name() string { return m.name }

func (m lifecycleModule) Init(ctx context.Context) error {
	if m.onInit != nil {
		m.onInit()
	}
	return nil
}

func (m lifecycleModule) Start(ctx context.Context) error {
	if m.onStart != nil {
		m.onStart()
	}
	<-ctx.Done()
	return nil
}

func (m lifecycleModule) Stop(ctx context.Context) error {
	if m.onStop != nil {
		m.onStop()
	}
	return nil
}

type blockUntilCanceled struct {
	name    string
	started chan struct{}
}

func (m blockUntilCanceled) Name() string                   { return m.name }
func (m blockUntilCanceled) Init(ctx context.Context) error { return nil }

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

func (m slowStopModule) Name() string                   { return m.name }
func (m slowStopModule) Init(ctx context.Context) error { return nil }

func (m slowStopModule) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (m slowStopModule) Stop(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

type orderedModule struct {
	name    string
	started chan struct{}
	onStop  func()
}

func (m orderedModule) Name() string                   { return m.name }
func (m orderedModule) Init(ctx context.Context) error { return nil }

func (m orderedModule) Start(ctx context.Context) error {
	select {
	case m.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return nil
}

func (m orderedModule) Stop(context.Context) error {
	if m.onStop != nil {
		m.onStop()
	}
	return nil
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

func TestReport_Err_JoinsHookErrors(t *testing.T) {
	t.Parallel()
	preErr := errors.New("pre failed")
	postErr := errors.New("post failed")
	r := supervisor.Report{
		Reason:         supervisor.ReasonSignal,
		PreStopErrors:  map[string]error{"bus.drain": preErr},
		PostStopErrors: map[string]error{"db.close": postErr},
	}
	err := r.Err()
	require.ErrorIs(t, err, preErr)
	require.ErrorIs(t, err, postErr)
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
	for i := 0; i < 200; i++ {
		sig := notifyTestSignal()
		ctx, stop := supervisor.NotifyContext(context.Background(), sig)

		require.NoError(t, sendNotifyTestSignal())
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
