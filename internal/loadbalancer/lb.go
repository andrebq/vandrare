package loadbalancer

import (
	"context"

	"github.com/andrebq/vandrare/internal/stack"
)

type (
	LB[T comparable] struct {
		ctx          context.Context
		targets      stack.S[chan<- T]
		cancel       context.CancelCauseFunc
		newTarget    chan chan<- T
		removeTarget chan chan<- T
		closeIfEmpty chan struct{}
		workload     chan T
	}
)

func NewLB[T comparable](parent context.Context) *LB[T] {
	lb := &LB[T]{
		newTarget:    make(chan chan<- T),
		removeTarget: make(chan chan<- T),
		closeIfEmpty: make(chan struct{}, 1),
		workload:     make(chan T),
	}
	lb.ctx, lb.cancel = context.WithCancelCause(parent)
	return lb
}

func (lb *LB[T]) Context() context.Context { return lb.ctx }
func (lb *LB[T]) Ch() chan<- T {
	return lb.workload
}

func (lb *LB[T]) New() chan T {
	ch := make(chan T)
	lb.Add(ch)
	return ch
}

func (lb *LB[T]) Add(conn chan<- T) {
	select {
	case lb.newTarget <- conn:
		println("added")
	case <-lb.ctx.Done():
	}
}

func (lb *LB[T]) Remove(conn chan<- T) {
	select {
	case lb.removeTarget <- conn:
	case <-lb.ctx.Done():
	}
}

func (lb *LB[T]) Run(ctx context.Context) error {
	var err error
	for err = lb.step(ctx); err == nil; err = lb.step(ctx) {
	}
	for _, v := range stack.Snapshot[chan<- T, []chan<- T](&lb.targets, nil) {
		close(v)
	}
	return err
}

func (lb *LB[T]) Close() error {
	lb.cancel(context.Canceled)
	return nil
}

func (lb *LB[T]) CloseIfEmpty() {
	select {
	case lb.closeIfEmpty <- struct{}{}:
	default:
	}
}

func (lb *LB[T]) step(ctx context.Context) error {
	wl := lb.workload
	var target chan<- T
	if lb.targets.Empty() {
		println("empty targets")
		wl = nil
	} else {
		target = lb.targets.Pop()
		defer lb.targets.Push(target)
	}
	select {
	case <-ctx.Done():
		lb.cancel(ctx.Err())
		return ctx.Err()
	case <-lb.ctx.Done():
		return lb.ctx.Err()
	case target := <-lb.newTarget:
		lb.targets.Push(target)
	case work := <-wl:
		lb.dispatch(ctx, target, work)
	case <-lb.closeIfEmpty:
		if lb.targets.Empty() {
			lb.cancel(nil)
		}
		return lb.ctx.Err()
	}
	return nil
}

func (lb *LB[T]) dispatch(ctx context.Context, target chan<- T, work T) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-lb.ctx.Done():
		return lb.ctx.Err()
	case target <- work:
		println("work dispatched")
		return nil
	}
}
