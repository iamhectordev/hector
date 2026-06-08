package supervisor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/iamhectordev/hector/pkg/telem"
)

// Supervisor runs one or more [Module] values until a terminal event.
type Supervisor struct {
	modules []Module
	cfg     config
}

// New builds a [Supervisor]. It copies the module slice.
func New(modules []Module, opts ...Option) (*Supervisor, error) {
	if len(modules) == 0 {
		return nil, fmt.Errorf("supervisor: at least one module is required")
	}
	cfg := config{}
	for _, o := range opts {
		if err := o(&cfg); err != nil {
			return nil, err
		}
	}
	if cfg.signalHandling && cfg.signalChan != nil {
		return nil, fmt.Errorf("supervisor: WithSignalHandling and WithSignalChan cannot be used together")
	}
	applyDefaults(&cfg)

	mods := append([]Module(nil), modules...)
	return &Supervisor{modules: mods, cfg: cfg}, nil
}

type modEvent struct {
	reason StopReason
	name   string
	err    error
	panicV any
}

type firstResult struct {
	reason StopReason
	name   string
	err    error
	panicV any
	sig    os.Signal
}

// Run blocks until a shutdown trigger occurs, then stops all modules.
//
// The shared context passed to [Module.Start] is canceled before [Module.Stop] runs.
func (s *Supervisor) Run(ctx context.Context) Report {
	if s.cfg.signalHandling {
		var stop context.CancelFunc
		ctx, stop = NotifyContext(ctx, s.cfg.signals...)
		defer stop()
	}

	if err := s.initAll(ctx); err != nil {
		return Report{Reason: ReasonInitError, Cause: err}
	}
	if err := s.runPostInitHooks(ctx); err != nil {
		return Report{Reason: ReasonInitError, Cause: err}
	}

	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	events := make(chan modEvent, len(s.modules)+1)

	for _, m := range s.modules {
		go s.runModule(m, runCtx, events)
	}

	first := s.waitFirstTerminal(ctx, s.signalPipe(), events)
	s.log(ctx).InfoContext(ctx, "shutdown",
		telem.Any("reason", first.reason),
		telem.String("trigger_module", first.name),
		telem.Any("err", first.err),
		telem.Any("signal", first.sig),
	)

	cancelRun()
	t0 := time.Now()
	preStopErrs := s.runHooks(ctx, s.cfg.preStopHooks)
	stopErrs := s.stopAll(ctx)
	postStopErrs := s.runHooks(ctx, s.cfg.postStopHooks)
	dur := time.Since(t0)

	rep := buildReport(first, preStopErrs, stopErrs, postStopErrs, dur)
	if len(rep.PreStopErrors) > 0 || len(rep.StopErrors) > 0 || len(rep.PostStopErrors) > 0 {
		s.log(ctx).ErrorContext(ctx, "stop completed with errors",
			telem.Any("pre_stop_errors", rep.PreStopErrors),
			telem.Any("stop_errors", rep.StopErrors),
			telem.Any("post_stop_errors", rep.PostStopErrors))
	} else {
		s.log(ctx).InfoContext(ctx, "stop completed",
			telem.Duration("shutdown_duration", rep.ShutdownDuration))
	}
	return rep
}

func (s *Supervisor) initAll(ctx context.Context) error {
	for _, m := range s.modules {
		if err := m.Init(ctx); err != nil {
			return fmt.Errorf("module %q init: %w", m.Name(), err)
		}
	}
	return nil
}

func (s *Supervisor) runPostInitHooks(ctx context.Context) error {
	for _, hook := range s.cfg.postInitHooks {
		if err := runHook(ctx, hook); err != nil {
			return fmt.Errorf("post-init hook %q: %w", hook.Name(), err)
		}
	}
	return nil
}

func (s *Supervisor) runModule(mod Module, runCtx context.Context, events chan<- modEvent) {
	defer func() {
		if pv := recover(); pv != nil {
			events <- modEvent{
				reason: ReasonModulePanic,
				name:   mod.Name(),
				panicV: pv,
			}
		}
	}()
	err := mod.Start(runCtx)
	if err != nil {
		events <- modEvent{reason: ReasonModuleError, name: mod.Name(), err: err}
		return
	}
	events <- modEvent{reason: ReasonModuleStopped, name: mod.Name()}
}

func (s *Supervisor) waitFirstTerminal(
	ctx context.Context,
	sigCh <-chan os.Signal,
	events <-chan modEvent,
) firstResult {
	for {
		select {
		case <-ctx.Done():
			cause := context.Cause(ctx)
			if sig, ok := signalFromCause(cause); ok {
				return firstResult{reason: ReasonSignal, sig: sig, err: cause}
			}
			if cause == nil {
				cause = ctx.Err()
			}
			return firstResult{reason: ReasonContextCanceled, err: cause}
		case sig := <-sigCh:
			return firstResult{reason: ReasonSignal, sig: sig}
		case ev := <-events:
			return modEventToFirst(ev)
		}
	}
}

func modEventToFirst(ev modEvent) firstResult {
	return firstResult{
		reason: ev.reason,
		name:   ev.name,
		err:    ev.err,
		panicV: ev.panicV,
	}
}

func (s *Supervisor) stopAll(ctx context.Context) map[string]error {
	errs := make(map[string]error)
	for i := len(s.modules) - 1; i >= 0; i-- {
		m := s.modules[i]
		stopCtx, cancel := s.newShutdownContext(ctx)
		err := m.Stop(stopCtx)
		deadlineExceeded := errors.Is(stopCtx.Err(), context.DeadlineExceeded)
		cancel()
		switch {
		case err != nil:
			errs[m.Name()] = err
		case deadlineExceeded:
			errs[m.Name()] = fmt.Errorf("module stop timed out after %s: %w", s.cfg.stopTimeout, context.DeadlineExceeded)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func (s *Supervisor) runHooks(ctx context.Context, hooks []ShutdownHook) map[string]error {
	errs := make(map[string]error)
	for _, hook := range hooks {
		hookCtx, cancel := s.newShutdownContext(ctx)
		err := runHook(hookCtx, hook)
		deadlineExceeded := errors.Is(hookCtx.Err(), context.DeadlineExceeded)
		cancel()
		switch {
		case err != nil:
			errs[hook.Name()] = err
		case deadlineExceeded:
			errs[hook.Name()] = fmt.Errorf("hook %q timed out after %s: %w", hook.Name(), s.cfg.stopTimeout, context.DeadlineExceeded)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func runHook(ctx context.Context, hook ShutdownHook) (err error) {
	defer func() {
		if pv := recover(); pv != nil {
			err = fmt.Errorf("hook %q panicked: %v", hook.Name(), pv)
		}
	}()
	return hook.Run(ctx)
}

// newShutdownContext creates the context used for shutdown work (hooks and module Stop).
// The input context is accepted so future versions can propagate trace/link metadata,
// while still deriving cancellation from a detached root to avoid skipped shutdown steps.
func (s *Supervisor) newShutdownContext(ctx context.Context) (context.Context, context.CancelFunc) {
	_ = ctx
	// Keep shutdown work decoupled from caller cancellation so hooks/stops still run.
	// Future: enrich this context with trace/link information for shutdown spans.
	return context.WithTimeout(context.Background(), s.cfg.stopTimeout)
}

func buildReport(first firstResult, preStopErrs, stopErrs, postStopErrs map[string]error, dur time.Duration) Report {
	rep := Report{
		Reason:           first.reason,
		TriggerModule:    first.name,
		Signal:           first.sig,
		ShutdownDuration: dur,
		PreStopErrors:    preStopErrs,
		StopErrors:       stopErrs,
		PostStopErrors:   postStopErrs,
	}
	switch first.reason {
	case ReasonSignal:
		rep.Cause = first.err
	case ReasonContextCanceled:
		rep.Cause = first.err
	case ReasonModuleError:
		rep.Cause = first.err
	case ReasonModulePanic:
		rep.PanicValue = first.panicV
	}
	return rep
}

func (s *Supervisor) signalPipe() <-chan os.Signal {
	if s.cfg.signalChan != nil {
		return s.cfg.signalChan
	}
	return nil
}

func (s *Supervisor) log(ctx context.Context) telem.ContextLogger {
	return telem.WrapLogger(ctx, s.cfg.logger)
}
