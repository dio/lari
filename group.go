package lari

import (
	"context"
	"sync"
)

type Group struct {
	mu     sync.RWMutex
	actors []actor
}

func (g *Group) Add(run func(context.Context) error, interrupt func(error)) {
	g.mu.Lock()
	g.actors = append(g.actors, newActor(run, interrupt))
	g.mu.Unlock()
}

func (g *Group) Run() error {
	g.mu.RLock()
	var runCtx runContext
	runCtx.registered = &sync.WaitGroup{}
	runCtx.started = &sync.WaitGroup{}
	for _, a := range g.actors {
		runCtx.units = append(runCtx.units, &unit{
			actor:  a,
			runCtx: &runCtx,
		})
	}
	g.mu.RUnlock()
	return runCtx.run()
}

type unit struct {
	runCtx *runContext
	actor  actor
}

type runContext struct {
	registered *sync.WaitGroup
	started    *sync.WaitGroup
	units      []*unit
}

func (r *runContext) run() error {
	if len(r.units) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	errors := make(chan error, len(r.units))

	for _, u := range r.units { // TODO(dio): Make sure we start in sequence.
		u.runCtx.registered.Add(1)
		u.runCtx.started.Add(1)
		go func(u *unit) {
			u.runCtx.registered.Done()
			u.runCtx.started.Done()

			errors <- u.actor.execute(ctx)
		}(u)
		u.runCtx.registered.Wait()
	}

	err := <-errors
	cancel()

	for _, u := range r.units {
		u.runCtx.started.Wait() // Best effort.
		u.actor.interrupt(err)
	}

	for i := 1; i < cap(r.units); i++ {
		<-errors
	}
	return err
}

func newActor(exec func(context.Context) error, cancel func(error)) actor {
	a := &actorImpl{
		exec:   exec,
		cancel: cancel,
	}
	a.wg.Add(1)
	return a
}

type actorImpl struct {
	wg     sync.WaitGroup
	exec   func(context.Context) error
	cancel func(error)
}

func (a *actorImpl) execute(ctx context.Context) error {
	a.wg.Done()
	return a.exec(ctx)
}

func (a *actorImpl) interrupt(err error) {
	a.wg.Wait()
	a.cancel(err)
}

type actor interface {
	execute(context.Context) error
	interrupt(error)
}
