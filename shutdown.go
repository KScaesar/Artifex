package Artifex

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func NewShutdown() *Shutdown {
	notify := make(chan os.Signal, 2)
	signal.Notify(notify, syscall.SIGINT, syscall.SIGTERM)
	return &Shutdown{
		done:      make(chan struct{}),
		sysSignal: notify,
		Logger:    DefaultLogger(),
	}
}

type Shutdown struct {
	done      chan struct{}
	cancel    context.CancelCauseFunc
	sysSignal chan os.Signal
	Logger    Logger

	stopQty     int
	names       []string
	stopActions []func() error
}

func (s *Shutdown) SetStopAction(name string, action func() error) *Shutdown {
	s.stopQty++
	s.names = append(s.names, name)
	s.stopActions = append(s.stopActions, action)
	return s
}

func (s *Shutdown) NotifyStop(cause error) {
	s.cancel(cause)
}

func (s *Shutdown) Wait() {
	<-s.done
}

func (s *Shutdown) Listen(ctx context.Context) *Shutdown {
	if ctx == nil {
		ctx = context.Background()
	}

	ctx, s.cancel = context.WithCancelCause(ctx)

	go func() {
		defer close(s.done)

		select {
		case sig := <-s.sysSignal:
			s.Logger.Info("receive signal: %v", sig)

		case <-ctx.Done():
			err := context.Cause(ctx)
			if errors.Is(err, context.Canceled) {
				s.Logger.Info("receive go context")
			} else {
				s.Logger.Error("receive go context: %v", err)
			}
		}

		s.Logger.Info("shutdown total service qty=%v", s.stopQty)
		wg := sync.WaitGroup{}
		for i := 0; i < s.stopQty; i++ {
			number := i + 1
			wg.Add(1)
			go func() {
				defer wg.Done()
				s.Logger.Info("number %v service %q shutdown start", number, s.names[number-1])
				err := s.stopActions[number-1]()
				if err != nil {
					s.Logger.Error("number %v service %q shutdown: %v", number, s.names[number-1], err)
				}
				s.Logger.Info("number %v service %q shutdown finish", number, s.names[number-1])
			}()
		}
		wg.Wait()
		s.Logger.Info("shutdown finish")
	}()

	return s
}
