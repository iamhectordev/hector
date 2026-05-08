package supervisor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"
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
func (s *Supervisor) Run(parent context.Context) Report {
	log := s.cfg.logger
	if s.cfg.signalHandling && !hasNotifyContext(parent) {
		var stop context.CancelFunc
		parent, stop = NotifyContext(parent, s.cfg.signals...)
		defer stop()
	}

	runCtx, cancelRun := context.WithCancel(parent)
	defer cancelRun()

	events := make(chan modEvent, len(s.modules)+1)

	for _, m := range s.modules {
		go s.runModule(m, runCtx, events)
	}

	first := s.waitFirstTerminal(parent, s.signalPipe(), events)
	if log != nil {
		log.InfoContext(parent, "supervisor: shutdown",
			slog.Any("reason", first.reason),
			slog.String("module", first.name),
			slog.Any("err", first.err),
			slog.Any("signal", first.sig),
		)
	}

	cancelRun()
	t0 := time.Now()
	stopErrs := s.stopAll()
	dur := time.Since(t0)

	rep := buildReport(first, stopErrs, dur)
	if log != nil {
		if len(rep.StopErrors) > 0 {
			log.ErrorContext(parent, "supervisor: stop completed with errors",
				slog.Any("stop_errors", rep.StopErrors))
		} else {
			log.InfoContext(parent, "supervisor: stop completed",
				slog.Duration("shutdown_duration", rep.ShutdownDuration))
		}
	}
	return rep
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
	parent context.Context,
	sigCh <-chan os.Signal,
	events <-chan modEvent,
) firstResult {
	for {
		select {
		case <-parent.Done():
			cause := context.Cause(parent)
			if sig, ok := signalFromCause(cause); ok {
				return firstResult{reason: ReasonSignal, sig: sig, err: cause}
			}
			if cause == nil {
				cause = parent.Err()
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

func (s *Supervisor) stopAll() map[string]error {
	errs := make(map[string]error)
	for i := len(s.modules) - 1; i >= 0; i-- {
		m := s.modules[i]
		stopCtx, cancel := context.WithTimeout(context.Background(), s.cfg.stopTimeout)
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

func buildReport(first firstResult, stopErrs map[string]error, dur time.Duration) Report {
	rep := Report{
		Reason:           first.reason,
		TriggerModule:    first.name,
		Signal:           first.sig,
		ShutdownDuration: dur,
		StopErrors:       stopErrs,
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
