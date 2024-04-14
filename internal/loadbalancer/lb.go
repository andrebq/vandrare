package loadbalancer

import (
	"context"
	"errors"
	"sync"

	"github.com/andrebq/vandrare/internal/set"
)

type (
	LB[T comparable] struct {
		mutext  sync.RWMutex
		workers set.RandomSet[chan<- T]
	}
)

var (
	errNoWorkers = errors.New("loadbalancer: no worker available")
)

func NewLB[T comparable](parent context.Context, seed int64) *LB[T] {
	return &LB[T]{
		workers: set.Random[chan<- T](seed),
	}
}

func (lb *LB[T]) Offer(ctx context.Context, work T) error {
	lb.mutext.Lock()
	worker, found := lb.workers.Pick()
	lb.mutext.Unlock()
	if !found {
		return errNoWorkers
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case worker <- work:
		return nil
	}
}

func (lb *LB[T]) New() chan T {
	ch := make(chan T)
	lb.mutext.Lock()
	lb.workers.Add(ch)
	lb.mutext.Unlock()
	return ch
}

func (lb *LB[T]) Remove(conn chan<- T) {
	lb.mutext.Lock()
	lb.workers.Remove(conn)
	lb.mutext.Unlock()
}

func (lb *LB[T]) Empty() bool {
	lb.mutext.Lock()
	sz := lb.workers.Len()
	lb.mutext.Unlock()
	return sz == 0
}
